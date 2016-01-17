package fi

import "github.com/golang/glog"

type node struct {
	unit     Unit
	children []*node
}

func (c *node) Add(node *node) {
	c.children = append(c.children, node)
}

func (n *node) Configure(c *RunContext) error {
	if n.unit != nil {
		glog.V(2).Infof("Configuring unit %v", n.unit)
		err := n.unit.Configure(c)
		if err != nil {
			return err
		}
	}
	for _, child := range n.children {
		childContext := c.buildChildContext(child)
		err := child.Configure(childContext)
		if err != nil {
			return err
		}
	}
	return nil
}
