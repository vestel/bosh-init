package compile_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	bmeventlog "github.com/cloudfoundry/bosh-micro-cli/eventlogger"
	bmrel "github.com/cloudfoundry/bosh-micro-cli/release"

	faketime "github.com/cloudfoundry/bosh-agent/time/fakes"

	fakebmcomp "github.com/cloudfoundry/bosh-micro-cli/cpideployer/compile/fakes"
	fakebmlog "github.com/cloudfoundry/bosh-micro-cli/eventlogger/fakes"
	fakebmreal "github.com/cloudfoundry/bosh-micro-cli/release/fakes"

	. "github.com/cloudfoundry/bosh-micro-cli/cpideployer/compile"
)

var _ = Describe("ReleaseCompiler", func() {
	var (
		release                 bmrel.Release
		releasePackagesCompiler ReleasePackagesCompiler
		da                      *fakebmreal.FakeDependencyAnalysis
		packageCompiler         *fakebmcomp.FakePackageCompiler
		eventLogger             *fakebmlog.FakeEventLogger
		timeService             *faketime.FakeService
	)

	BeforeEach(func() {
		da = fakebmreal.NewFakeDependencyAnalysis()
		packageCompiler = fakebmcomp.NewFakePackageCompiler()
		eventLogger = fakebmlog.NewFakeEventLogger()
		timeService = &faketime.FakeService{}
		releasePackagesCompiler = NewReleasePackagesCompiler(da, packageCompiler, eventLogger, timeService)
		release = bmrel.Release{}
	})

	Context("Compile", func() {
		Context("when there is a release", func() {
			var expectedPackages []*bmrel.Package
			var package1, package2 bmrel.Package

			BeforeEach(func() {
				package1 = bmrel.Package{Name: "fake-package-1", Fingerprint: "fake-fingerprint-1"}
				package2 = bmrel.Package{Name: "fake-package-2", Fingerprint: "fake-fingerprint-2"}

				expectedPackages = []*bmrel.Package{&package1, &package2}

				da.DeterminePackageCompilationOrderResult = []*bmrel.Package{
					&package1,
					&package2,
				}
			})

			It("determines the order to compile packages", func() {
				err := releasePackagesCompiler.Compile(release)
				Expect(err).NotTo(HaveOccurred())
				Expect(da.DeterminePackageCompilationOrderRelease).To(Equal(release))
			})

			It("compiles each package", func() {
				err := releasePackagesCompiler.Compile(release)
				Expect(err).NotTo(HaveOccurred())
				Expect(packageCompiler.CompilePackages).To(Equal(expectedPackages))
			})

			It("compiles each package and returns error for first package", func() {
				packageCompiler.CompileError = errors.New("Compilation failed")
				err := releasePackagesCompiler.Compile(release)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Package `fake-package-1' compilation failed"))
			})

			It("logs start and stop events to the eventLogger", func() {
				pkg1Start := time.Now()
				pkg1Finish := pkg1Start.Add(1 * time.Second)
				timeService.NowTimes = []time.Time{pkg1Start, pkg1Finish}

				err := releasePackagesCompiler.Compile(release)
				Expect(err).ToNot(HaveOccurred())

				expectedStartEvent := bmeventlog.Event{
					Time:  pkg1Start,
					Stage: "compiling packages",
					Total: 2,
					Task:  "fake-package-1/fake-fingerprint-1",
					Index: 1,
					State: "started",
				}

				expectedFinishEvent := bmeventlog.Event{
					Time:  pkg1Finish,
					Stage: "compiling packages",
					Total: 2,
					Task:  "fake-package-1/fake-fingerprint-1",
					Index: 1,
					State: "finished",
				}

				Expect(eventLogger.LoggedEvents).To(ContainElement(expectedStartEvent))
				Expect(eventLogger.LoggedEvents).To(ContainElement(expectedFinishEvent))
			})

			It("logs events for each of the packages", func() {
				err := releasePackagesCompiler.Compile(release)
				Expect(err).ToNot(HaveOccurred())
				Expect(eventLogger.LoggedEvents).To(HaveLen(4))
			})

			It("logs failure event", func() {
				pkg1Start := time.Now()
				pkg1Fail := pkg1Start.Add(1 * time.Second)
				timeService.NowTimes = []time.Time{pkg1Start, pkg1Fail}

				packageCompiler.CompileError = errors.New("Compilation failed")
				err := releasePackagesCompiler.Compile(release)
				Expect(err).To(HaveOccurred())

				expectedFailEvent := bmeventlog.Event{
					Time:    pkg1Fail,
					Stage:   "compiling packages",
					Total:   2,
					Task:    "fake-package-1/fake-fingerprint-1",
					Index:   1,
					State:   "failed",
					Message: "Compilation failed",
				}

				Expect(eventLogger.LoggedEvents).To(ContainElement(expectedFailEvent))
			})

			It("stops compiling after the first failure", func() {
				packageCompiler.CompileError = errors.New("Compilation failed")
				err := releasePackagesCompiler.Compile(release)
				Expect(err).To(HaveOccurred())
				Expect(len(packageCompiler.CompilePackages)).To(Equal(1))
			})

			Context("when adding a started event fails", func() {
				BeforeEach(func() {
					eventLogger.AddEventErrors[bmeventlog.Started] = errors.New("fake-add-event-error")
				})

				It("returns error", func() {
					err := releasePackagesCompiler.Compile(release)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Logging event"))
				})
			})

			Context("when adding a failed event fails", func() {
				BeforeEach(func() {
					packageCompiler.CompileError = errors.New("Compilation failed")
					eventLogger.AddEventErrors[bmeventlog.Failed] = errors.New("fake-add-event-error")
				})

				It("returns error", func() {
					err := releasePackagesCompiler.Compile(release)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Logging event"))
				})
			})

			Context("when adding a finished event fails", func() {
				BeforeEach(func() {
					eventLogger.AddEventErrors[bmeventlog.Finished] = errors.New("fake-add-event-error")
				})

				It("returns error", func() {
					err := releasePackagesCompiler.Compile(release)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Logging event"))
				})
			})
		})
	})
})
