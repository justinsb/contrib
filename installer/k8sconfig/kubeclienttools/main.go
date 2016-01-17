package kubeclienttools

import (
	"github.com/kubernetes/contrib/installer/pkg/fi"
	"github.com/kubernetes/contrib/installer/pkg/files"
)

type KubeClientTools struct {
	fi.StructuralUnit
}

func (k *KubeClientTools) Add(c *fi.BuildContext) {
	c.Add(files.Path("/usr/local/bin/kubectl").WithContents(fi.Resource("kubectl")).WithMode(0755))
}
