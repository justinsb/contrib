package kup

import "k8s.io/contrib/installer/pkg/fi"

type Cluster struct {
	fi.StructuralUnit
}

func (k *Cluster) Add(c *fi.BuildContext) {
	c.Add(&VPC{})
}
