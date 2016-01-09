package ntp

import (
	"github.com/kubernetes/contrib/installer/pkg/fi"
	"github.com/kubernetes/contrib/installer/pkg/packages"
	"github.com/kubernetes/contrib/installer/pkg/services"
)

func Add(context *fi.Context) {
	context.Add(packages.Installed("ntp"))
	context.Add(services.Running("ntp"))
}
