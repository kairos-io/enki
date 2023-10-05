package action

type FindMatchingFrameworkAction struct{}

func NewFindMatchingFrameworkAction() *FindMatchingFrameworkAction {
	return &FindMatchingFrameworkAction{}
}

func (a *FindMatchingFrameworkAction) Run() (framework string, err error) {
	return
}
