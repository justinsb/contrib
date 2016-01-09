package debianautoupgrades

import (
	"github.com/kubernetes/contrib/installer/pkg/fi"
	"github.com/kubernetes/contrib/installer/pkg/files"
	"github.com/kubernetes/contrib/installer/pkg/packages"
)

func Add(c *fi.Context) {
	if c.OS().IsDebian() {
		c.Add(packages.Installed("unattended-upgrades"))

		c.Add(files.Path("/etc/apt/apt.conf.d/20auto-upgrades").WithContents(buildAutoUpgradesConf))
	}
}

func buildAutoUpgradesConf() (string, error) {
	return `
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";

APT::Periodic::AutocleanInterval "7";
`, nil
}
