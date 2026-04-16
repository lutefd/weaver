package stack

type Dependency struct {
	Branch string
	Parent string
}

type StackNode struct {
	Branch string
}

type StackHealthState string

const (
	HealthClean        StackHealthState = "clean"
	HealthOutdated     StackHealthState = "outdated"
	HealthConflictRisk StackHealthState = "conflict risk"
)

type StackHealth struct {
	State  StackHealthState
	Behind int
}

type UpstreamHealthState string

const (
	UpstreamCurrent  UpstreamHealthState = "up to date"
	UpstreamBehind   UpstreamHealthState = "behind upstream"
	UpstreamAhead    UpstreamHealthState = "ahead of upstream"
	UpstreamDiverged UpstreamHealthState = "diverged"
	UpstreamMissing  UpstreamHealthState = "no upstream"
)

type UpstreamHealth struct {
	State  UpstreamHealthState
	Ahead  int
	Behind int
}
