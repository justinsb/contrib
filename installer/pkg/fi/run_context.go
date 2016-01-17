package fi

type RunContext struct {
	*Context

	parent *RunContext
	node   *node
}

func (c *RunContext) buildChildContext(n *node) *RunContext {
	child := &RunContext{
		Context: c.Context,
		parent:  c,
		node:    n,
	}
	return child
}

func (c *RunContext) Configure() error {
	return c.node.Configure(c)
}
