package k8sconfig

import (
	"k8s.io/contrib/installer/pkg/fi"
	"k8s.io/contrib/installer/pkg/packages"
)

type Base struct {
	fi.StructuralUnit
}

func (b *Base) Add(c *fi.BuildContext) {
	c.Add(packages.Installed("curl"))
	if c.OS().IsRedhat() {
		c.Add(packages.Installed("python"))
		c.Add(packages.Installed("git"))
	} else {
		c.Add(packages.Installed("apt-transport-https"))
		c.Add(packages.Installed("python-apt"))
		c.Add(packages.Installed("nfs-common"))
		c.Add(packages.Installed("socat"))

		// Ubuntu installs netcat-openbsd by default, but on GCE/Debian netcat-traditional is installed.
		// They behave slightly differently.
		// For sanity, we try to maek sure we have the same netcat on all OSes (#15166)
		c.Add(packages.Installed("netcat-traditional"))
	}
}