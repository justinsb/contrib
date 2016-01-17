package fi

type BuildContext struct {
	*Context
	node *node
}

type Builder interface {
	Add(*BuildContext)
}

func (b *BuildContext) Add(unit Unit) {
	childNode := b.newNode(unit)

	builder, ok := unit.(Builder)
	if ok {
		childContext := b.createChildContext(childNode)
		builder.Add(childContext)
	}

	b.node.Add(childNode)
}

func (b *BuildContext) createChildContext(childNode *node) *BuildContext {
	bc := &BuildContext{
		Context: b.Context,
		node:    childNode,
	}
	return bc
}
