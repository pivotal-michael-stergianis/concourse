package engine

import (
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
)

type getDelegate struct {
	exec.BuildStepDelegate

	build       db.Build
	eventOrigin event.Origin
}

func NewGetDelegate(build db.Build, planID atc.PlanID, clock clock.Clock) exec.GetDelegate {
	return &getDelegate{
		BuildStepDelegate: NewBuildStepDelegate(build, planID, clock),

		build: build,
		eventOrigin: event.Origin{
			ID: event.OriginID(planID),
		},
	}
}

func (d *getDelegate) Initializing(logger lager.Logger) {
	err := d.build.SaveEvent(event.InitializeGet{
		Origin: d.eventOrigin,
		Time:   time.Now().Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-get-event", err)
		return
	}

	logger.Debug("initializing")
}

func (d *getDelegate) Starting(logger lager.Logger) {
	err := d.build.SaveEvent(event.StartGet{
		Time:   time.Now().Unix(),
		Origin: d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-start-get-event", err)
		return
	}

	logger.Info("starting")
}

func (d *getDelegate) Finished(logger lager.Logger, exitStatus exec.ExitStatus, info exec.VersionInfo) {
	err := d.build.SaveEvent(event.FinishGet{
		Time:            time.Now().Unix(),
		Origin:          d.eventOrigin,
		ExitStatus:      int(exitStatus),
		FetchedVersion:  info.Version,
		FetchedMetadata: info.Metadata,
	})
	if err != nil {
		logger.Error("failed-to-save-finish-get-event", err)
		return
	}

	logger.Info("finished", lager.Data{"exit-status": exitStatus})
}
