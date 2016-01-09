//go:generate esc -o res.go -pkg kubenodeunpacker res
package kubenodeunpacker

import (
	"github.com/kubernetes/contrib/installer/pkg/fi"
	"github.com/kubernetes/contrib/installer/pkg/files"
	"github.com/kubernetes/contrib/installer/pkg/services"
)

func Add(context *fi.Context) {
	context.Add(files.Path("/etc/kubernetes/kube-node-unpacker.sh").WithContents(fi.FSResource(FS, "kube-node-unpacker.sh")).WithMode(0755))

	if context.Cloud().IsGCE() {
		panic("GCE not supported in kubenodeunpacker")
	} else {
		context.Add(files.Path("/srv/salt/kube-bins/kube-proxy.tar").WithContents(fi.Resource("kube-proxy.tar")))
	}

	service := services.Running("kube-node-unpacker")
	service.RunOnce = true
	service.Description = "Kubernetes Node Unpacker"
	service.Documentation = "https://github.com/GoogleCloudPlatform/kubernetes"
	service.Exec = "/etc/kubernetes/kube-node-unpacker.sh"
	context.Add(service)
}
