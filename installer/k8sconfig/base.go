package k8sconfig

import (
	"github.com/kubernetes/contrib/installer/pkg/fi"
	"github.com/kubernetes/contrib/installer/pkg/packages"
)

func addBase(c *fi.Context) {
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
