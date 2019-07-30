package exec

import (
	"context"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
)

//go:generate counterfeiter . PutDelegate

type PutDelegate interface {
	BuildStepDelegate

	Initializing(lager.Logger)
	Starting(lager.Logger)
	Finished(lager.Logger, ExitStatus, runtime.VersionResult)
	SaveOutput(lager.Logger, atc.PutPlan, atc.Source, atc.VersionedResourceTypes, runtime.VersionResult)
}

// PutStep produces a resource version using preconfigured params and any data
// available in the worker.ArtifactRepository.
type PutStep struct {
	planID                atc.PlanID
	plan                  atc.PutPlan
	metadata              StepMetadata
	containerMetadata     db.ContainerMetadata
	resourceFactory       resource.ResourceFactory
	resourceConfigFactory db.ResourceConfigFactory
	strategy              worker.ContainerPlacementStrategy
	workerClient      	  worker.Client
	delegate              PutDelegate
	succeeded             bool
}

func NewPutStep(
	planID atc.PlanID,
	plan atc.PutPlan,
	metadata StepMetadata,
	containerMetadata db.ContainerMetadata,
	resourceFactory resource.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	strategy worker.ContainerPlacementStrategy,
	workerClient worker.Client,
	delegate PutDelegate,
) *PutStep {
	return &PutStep{
		planID:                planID,
		plan:                  plan,
		metadata:              metadata,
		containerMetadata:     containerMetadata,
		resourceFactory:       resourceFactory,
		resourceConfigFactory: resourceConfigFactory,
		workerClient:     	   workerClient,
		strategy:              strategy,
		delegate:              delegate,
	}
}

// Run chooses a worker that supports the step's resource type and creates a
// container.
//
// All worker.ArtifactSources present in the worker.ArtifactRepository are then brought into
// the container, using volumes if possible, and streaming content over if not.
//
// The resource's put script is then invoked. If the context is canceled, the
// script will be interrupted.
func (step *PutStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("put-step", lager.Data{
		"step-name": step.plan.Name,
		"job-id":    step.metadata.JobID,
	})

	//step.delegate.Initializing(logger)

	variables := step.delegate.Variables()

	source, err := creds.NewSource(variables, step.plan.Source).Evaluate()
	if err != nil {
		return err
	}

	params, err := creds.NewParams(variables, step.plan.Params).Evaluate()
	if err != nil {
		return err
	}

	resourceTypes, err := creds.NewVersionedResourceTypes(variables, step.plan.VersionedResourceTypes).Evaluate()
	if err != nil {
		return err
	}

	var putInputs PutInputs
	if step.plan.Inputs == nil {
		// Put step defaults to all inputs if not specified
		putInputs = NewAllInputs()
	} else if step.plan.Inputs.All {
		putInputs = NewAllInputs()
	} else {
		// Covers both cases where inputs are specified and when there are no
		// inputs specified and "all" field is given a false boolean, which will
		// result in no inputs attached
		putInputs = NewSpecificInputs(step.plan.Inputs.Specified)
	}

	containerInputs, err := putInputs.FindAll(state.Artifacts())
	if err != nil {
		return err
	}

	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: step.plan.Type,
		},
		Tags:   step.plan.Tags,
		TeamID: step.metadata.TeamID,

		Dir: step.containerMetadata.WorkingDirectory,

		Env: step.metadata.Env(),

		Inputs: containerInputs,
	}

	workerSpec := worker.WorkerSpec{
		ResourceType:  step.plan.Type,
		Tags:          step.plan.Tags,
		TeamID:        step.metadata.TeamID,
		ResourceTypes: resourceTypes,
	}

	owner := db.NewBuildStepContainerOwner(step.metadata.BuildID, step.planID, step.metadata.TeamID)

	containerSpec.BindMounts = []worker.BindMountSource{
		&worker.CertsVolumeMount{Logger: logger},
	}

	imageSpec := worker.ImageFetcherSpec{
		ResourceTypes: resourceTypes,
		Delegate:      step.delegate,
	}


	events := make(chan runtime.Event, 1)
	//go func(logger lager.Logger, events chan runtime.Event, delegate PutDelegate) {
	//	for {
	//		ev := <-events
	//		switch {
	//		case ev.EventType == runtime.InitializingEvent:
	//			step.delegate.Initializing(logger)
	//
	//		case ev.EventType == runtime.StartingEvent:
	//			step.delegate.Starting(logger)
	//
	//		case ev.EventType == runtime.FinishedEvent:
	//			step.delegate.Finished(logger, ExitStatus(ev.ExitStatus))
	//
	//		default:
	//			return
	//		}
	//	}
	//}(logger, events, step.delegate)

	step.delegate.Initializing(logger)

	resourceDir := resource.ResourcesDir("put")

	versionResult, err := step.workerClient.RunPutStep(
		ctx,
		logger,
		owner,
		containerSpec,
		workerSpec,
		source,
		params,
		step.strategy,
		step.containerMetadata,
		imageSpec,
		resourceDir,
		runtime.IOConfig{
			Stdout: step.delegate.Stdout(),
			Stderr: step.delegate.Stderr(),
		},
		events,
	)

	//chosenWorker, err := step.pool.FindOrChooseWorkerForContainer(
	//	ctx,
	//	logger,
	//	owner,
	//	containerSpec,
	//	workerSpec,
	//	step.strategy,
	//)
	//if err != nil {
	//	return err
	//}
	//
	//containerSpec.BindMounts = []worker.BindMountSource{
	//	&worker.CertsVolumeMount{Logger: logger},
	//}
	//
	//container, err := chosenWorker.FindOrCreateContainer(
	//	ctx,
	//	logger,
	//	step.delegate,
	//	owner,
	//	step.containerMetadata,
	//	containerSpec,
	//	resourceTypes,
	//)
	//if err != nil {
	//	return err
	//}
	//
	//step.delegate.Starting(logger)
	//
	//putResource := step.resourceFactory.NewResourceForContainer(container)
	//versionResult, err := putResource.Put(
	//	ctx,
	//	resource.IOConfig{
	//		Stdout: step.delegate.Stdout(),
	//		Stderr: step.delegate.Stderr(),
	//	},
	//	source,
	//	params,
	//)
	//
	if err != nil {
		logger.Error("failed-to-put-resource", err)

		if err, ok := err.(resource.ErrResourceScriptFailed); ok {
			step.delegate.Finished(logger, ExitStatus(err.ExitStatus), runtime.VersionResult{})
			return nil
		}

		return err
	}

	if step.plan.Resource != "" {
		step.delegate.SaveOutput(logger, step.plan, source, resourceTypes, versionResult)
	}

	state.StoreResult(step.planID, versionResult)

	step.succeeded = true

	step.delegate.Finished(logger, 0, versionResult)

	return nil

}

// Succeeded returns true if the resource script exited successfully.
func (step *PutStep) Succeeded() bool {
	return step.succeeded
}
