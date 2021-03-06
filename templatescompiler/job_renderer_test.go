package templatescompiler_test

import (
	"path/filepath"

	bosherr "github.com/cloudfoundry/bosh-init/internal/github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-init/internal/github.com/cloudfoundry/bosh-utils/logger"
	biproperty "github.com/cloudfoundry/bosh-init/internal/github.com/cloudfoundry/bosh-utils/property"
	fakesys "github.com/cloudfoundry/bosh-init/internal/github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/cloudfoundry/bosh-init/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-init/internal/github.com/onsi/gomega"
	bireljob "github.com/cloudfoundry/bosh-init/release/job"
	bierbrenderer "github.com/cloudfoundry/bosh-init/templatescompiler/erbrenderer"

	fakebirender "github.com/cloudfoundry/bosh-init/templatescompiler/erbrenderer/fakes"

	. "github.com/cloudfoundry/bosh-init/templatescompiler"
)

var _ = Describe("JobRenderer", func() {
	var (
		jobRenderer      JobRenderer
		fakeERBRenderer  *fakebirender.FakeERBRenderer
		job              bireljob.Job
		context          bierbrenderer.TemplateEvaluationContext
		fs               *fakesys.FakeFileSystem
		jobProperties    biproperty.Map
		globalProperties biproperty.Map
		srcPath          string
		dstPath          string
	)

	BeforeEach(func() {
		srcPath = "fake-src-path"
		dstPath = "fake-dst-path"
		jobProperties = biproperty.Map{
			"fake-property-key": "fake-job-property-value",
		}

		globalProperties = biproperty.Map{
			"fake-property-key": "fake-global-property-value",
		}

		job = bireljob.Job{
			Templates: map[string]string{
				"director.yml.erb": "config/director.yml",
			},
			ExtractedPath: srcPath,
		}

		logger := boshlog.NewLogger(boshlog.LevelNone)

		context = NewJobEvaluationContext(job, jobProperties, globalProperties, "fake-deployment-name", logger)

		fakeERBRenderer = fakebirender.NewFakeERBRender()

		fs = fakesys.NewFakeFileSystem()
		jobRenderer = NewJobRenderer(fakeERBRenderer, fs, logger)

		fakeERBRenderer.SetRenderBehavior(
			filepath.Join(srcPath, "templates/director.yml.erb"),
			filepath.Join(dstPath, "config/director.yml"),
			context,
			nil,
		)

		fakeERBRenderer.SetRenderBehavior(
			filepath.Join(srcPath, "monit"),
			filepath.Join(dstPath, "monit"),
			context,
			nil,
		)

		fs.TempDirDir = dstPath
	})

	AfterEach(func() {
		err := fs.RemoveAll(dstPath)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Render", func() {
		It("renders job templates", func() {
			renderedjob, err := jobRenderer.Render(job, jobProperties, globalProperties, "fake-deployment-name")
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeERBRenderer.RenderInputs).To(Equal([]fakebirender.RenderInput{
				{
					SrcPath: filepath.Join(srcPath, "templates/director.yml.erb"),
					DstPath: filepath.Join(renderedjob.Path(), "config/director.yml"),
					Context: context,
				},
				{
					SrcPath: filepath.Join(srcPath, "monit"),
					DstPath: filepath.Join(renderedjob.Path(), "monit"),
					Context: context,
				},
			}))
		})

		Context("when rendering fails", func() {
			BeforeEach(func() {
				fakeERBRenderer.SetRenderBehavior(
					filepath.Join(srcPath, "templates/director.yml.erb"),
					filepath.Join(dstPath, "config/director.yml"),
					context,
					bosherr.Error("fake-template-render-error"),
				)
			})

			It("returns an error", func() {
				_, err := jobRenderer.Render(job, jobProperties, globalProperties, "fake-deployment-name")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-template-render-error"))
			})
		})
	})
})
