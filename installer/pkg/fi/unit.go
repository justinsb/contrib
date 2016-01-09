package fi

type Unit interface {
	Configure(c *Context) error
}
