package stack

type Dependency struct {
	Branch string
	Parent string
}

type StackNode struct {
	Branch string
}

type StackHealth string

const (
	HealthClean        StackHealth = "clean"
	HealthNeedsRebase  StackHealth = "needs rebase"
	HealthConflictRisk StackHealth = "conflict risk"
)
