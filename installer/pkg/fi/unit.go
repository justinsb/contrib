package fi

type Unit interface {
	Run(c *RunContext) error
	Path() string
}
