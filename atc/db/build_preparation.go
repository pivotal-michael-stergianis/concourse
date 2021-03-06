package db

import "fmt"

type BuildPreparationStatus string

const (
	BuildPreparationStatusUnknown     BuildPreparationStatus = "unknown"
	BuildPreparationStatusBlocking    BuildPreparationStatus = "blocking"
	BuildPreparationStatusNotBlocking BuildPreparationStatus = "not_blocking"
)

type MissingInputReasons map[string]string

const (
	NoVersionsSatisfiedPassedConstraints string = "no versions satisfy passed constraints"
	NoVersionsAvailable                  string = "no versions available"
	NoResourceCheckFinished              string = "checking for latest available versions"
	PinnedVersionUnavailable             string = "pinned version %s is not available"
)

func (mir MissingInputReasons) RegisterPassedConstraint(inputName string) {
	mir[inputName] = NoVersionsSatisfiedPassedConstraints
}

func (mir MissingInputReasons) RegisterNoVersions(inputName string) {
	mir[inputName] = NoVersionsAvailable
}

func (mir MissingInputReasons) RegisterNoResourceCheckFinished(inputName string) {
	mir[inputName] = NoResourceCheckFinished
}

func (mir MissingInputReasons) RegisterPinnedVersionUnavailable(inputName string, version string) {
	mir[inputName] = fmt.Sprintf(PinnedVersionUnavailable, version)
}

type BuildPreparation struct {
	BuildID             int
	PausedPipeline      BuildPreparationStatus
	PausedJob           BuildPreparationStatus
	MaxRunningBuilds    BuildPreparationStatus
	Inputs              map[string]BuildPreparationStatus
	InputsSatisfied     BuildPreparationStatus
	MissingInputReasons MissingInputReasons
}
