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
