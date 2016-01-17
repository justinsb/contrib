package ntp

import (
	"github.com/kubernetes/contrib/installer/pkg/fi"
	"github.com/kubernetes/contrib/installer/pkg/packages"
	"github.com/kubernetes/contrib/installer/pkg/services"
)

type Ntp struct {
	fi.StructuralUnit
}

func (n *Ntp) Add(c *fi.BuildContext) {
	c.Add(packages.Installed("ntp"))
	c.Add(services.Running("ntp"))
}
