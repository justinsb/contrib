package fi

type Unit interface {
	Configure(c *RunContext) error
}
