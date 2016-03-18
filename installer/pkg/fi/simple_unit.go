package fi

type SimpleUnit struct {
	parent Unit
	path string
}

var _ Unit = &SimpleUnit{}

func (u *SimpleUnit) Run(c *RunContext) error {
	return nil
}

func (u *SimpleUnit) Path() string {
	if u.parent == nil {
		return u.path
	}
	return u.parent.Path() + "/" + u.path
}