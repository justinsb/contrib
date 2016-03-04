package ntp

import (
	"k8s.io/contrib/installer/pkg/fi"
	"k8s.io/contrib/installer/pkg/packages"
	"k8s.io/contrib/installer/pkg/services"
)

type Ntp struct {
	fi.StructuralUnit
}

func (n *Ntp) Add(c *fi.BuildContext) {
	c.Add(packages.Installed("ntp"))
	c.Add(services.Running("ntp"))
}
