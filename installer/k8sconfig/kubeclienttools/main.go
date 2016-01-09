package kubeclienttools

import (
	"github.com/kubernetes/contrib/installer/pkg/fi"
	"github.com/kubernetes/contrib/installer/pkg/files"
)

func Add(context *fi.Context) {
	context.Add(files.Path("/usr/local/bin/kubectl").WithContents(fi.Resource("kubectl")).WithMode(0755))
}
