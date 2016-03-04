//go:generate esc -o res.go -pkg kubenodeunpacker --prefix res/ res
package kubenodeunpacker

import (
	"k8s.io/contrib/installer/pkg/fi"
	"k8s.io/contrib/installer/pkg/files"
	"k8s.io/contrib/installer/pkg/services"
)

type KubeNodeUnpacker struct {
	fi.StructuralUnit
}

func (k *KubeNodeUnpacker) Add(c *fi.BuildContext) {
	c.Add(files.Path("/etc/kubernetes/kube-node-unpacker.sh").WithContents(fi.EmbeddedResource(FS(false), "/kube-node-unpacker.sh")).WithMode(0755))

	if c.Cloud().ProviderID() != "aws" {
		panic("GCE not supported in kubenodeunpacker")
	} else {
		c.Add(files.Path("/srv/salt/kube-bins/kube-proxy.tar").WithContents(c.Resource("kube-proxy.tar")))
	}

	service := services.Running("kube-node-unpacker")
	service.RunOnce = true
	service.Description = "Kubernetes Node Unpacker"
	service.Documentation = "https://github.com/GoogleCloudPlatform/kubernetes"
	service.Exec = "/etc/kubernetes/kube-node-unpacker.sh"
	c.Add(service)
}
