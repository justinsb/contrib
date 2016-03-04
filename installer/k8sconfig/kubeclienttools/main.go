package kubeclienttools

import (
	"k8s.io/contrib/installer/pkg/fi"
	"k8s.io/contrib/installer/pkg/files"
)

type KubeClientTools struct {
	fi.StructuralUnit
}

func (k *KubeClientTools) Add(c *fi.BuildContext) {
	c.Add(files.Path("/usr/local/bin/kubectl").WithContents(c.Resource("kubectl")).WithMode(0755))
}
