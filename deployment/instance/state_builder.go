package instance

import (
	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshuuid "github.com/cloudfoundry/bosh-agent/uuid"

	bmblobstore "github.com/cloudfoundry/bosh-micro-cli/blobstore"
	bmdeplmanifest "github.com/cloudfoundry/bosh-micro-cli/deployment/manifest"
	bmstemcell "github.com/cloudfoundry/bosh-micro-cli/deployment/stemcell"
	bmreljob "github.com/cloudfoundry/bosh-micro-cli/release/job"
	bmtemplate "github.com/cloudfoundry/bosh-micro-cli/templatescompiler"
)

type StateBuilder interface {
	Build(jobName string, jobID int, deploymentManifest bmdeplmanifest.Manifest, stemcellApplySpec bmstemcell.ApplySpec) (State, error)
}

type stateBuilder struct {
	releaseJobResolver        bmreljob.Resolver
	jobRenderer               bmtemplate.JobListRenderer
	renderedJobListCompressor bmtemplate.RenderedJobListCompressor
	blobstore                 bmblobstore.Blobstore
	uuidGenerator             boshuuid.Generator
	logger                    boshlog.Logger
	logTag                    string
}

func NewStateBuilder(
	releaseJobResolver bmreljob.Resolver,
	jobRenderer bmtemplate.JobListRenderer,
	renderedJobListCompressor bmtemplate.RenderedJobListCompressor,
	blobstore bmblobstore.Blobstore,
	uuidGenerator boshuuid.Generator,
	logger boshlog.Logger,
) StateBuilder {
	return &stateBuilder{
		releaseJobResolver:        releaseJobResolver,
		jobRenderer:               jobRenderer,
		renderedJobListCompressor: renderedJobListCompressor,
		blobstore:                 blobstore,
		uuidGenerator:             uuidGenerator,
		logger:                    logger,
		logTag:                    "stateBuilder",
	}
}

func (b *stateBuilder) Build(jobName string, jobID int, deploymentManifest bmdeplmanifest.Manifest, stemcellApplySpec bmstemcell.ApplySpec) (State, error) {
	deploymentJob, found := deploymentManifest.FindJobByName(jobName)
	if !found {
		return nil, bosherr.Errorf("Job '%s' not found in deployment manifest", jobName)
	}

	releaseJobs, err := b.releaseJobResolver.ResolveEach(deploymentJob.ReleaseJobReferences())
	if err != nil {
		return nil, err
	}

	jobProperties, err := deploymentJob.Properties()
	if err != nil {
		return nil, bosherr.WrapError(err, "Stringifying job properties")
	}

	renderedJobList, err := b.jobRenderer.Render(releaseJobs, jobProperties, deploymentManifest.Name)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Rendering templates for job '%s'", jobName)
	}
	defer renderedJobList.DeleteSilently()

	renderedJobListArchive, err := b.renderedJobListCompressor.Compress(renderedJobList)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Compressing templates for job '%s'", jobName)
	}
	defer renderedJobListArchive.DeleteSilently()

	blobID, err := b.uploadJobTemplateListArchive(renderedJobListArchive)
	if err != nil {
		return nil, err
	}

	networks, err := deploymentManifest.NetworksSpec(deploymentJob.Name)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Finding networks for job '%s", jobName)
	}

	packageBlobs := []PackageBlob{}
	for _, packageBlob := range stemcellApplySpec.Packages {
		packageBlobs = append(packageBlobs, PackageBlob{
			Name:        packageBlob.Name,
			Version:     packageBlob.Version,
			SHA1:        packageBlob.SHA1,
			BlobstoreID: packageBlob.BlobstoreID,
		})
	}

	renderedJobListBlob := RenderedJobListBlob{
		BlobstoreID: blobID,
		SHA1:        renderedJobListArchive.SHA1(),
	}

	return &state{
		deploymentName:      deploymentManifest.Name,
		name:                jobName,
		id:                  jobID,
		networks:            networks,
		jobs:                releaseJobs,
		packageBlobs:        packageBlobs,
		renderedJobListBlob: renderedJobListBlob,
		stateHash:           renderedJobListArchive.Fingerprint(),
	}, nil
}

func (b *stateBuilder) uploadJobTemplateListArchive(
	renderedJobListArchive bmtemplate.RenderedJobListArchive,
) (blobID string, err error) {
	b.logger.Debug(b.logTag, "Saving job template list archive to blobstore")

	blobID, err = b.uuidGenerator.Generate()
	if err != nil {
		return "", bosherr.WrapError(err, "Generating Blob ID")
	}

	err = b.blobstore.Save(renderedJobListArchive.Path(), blobID)
	if err != nil {
		return "", bosherr.WrapErrorf(err, "Uploading blob at '%s'", renderedJobListArchive.Path())
	}

	return blobID, nil
}
