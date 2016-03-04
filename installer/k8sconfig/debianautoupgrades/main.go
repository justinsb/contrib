package debianautoupgrades

import (
	"k8s.io/contrib/installer/pkg/fi"
	"k8s.io/contrib/installer/pkg/files"
	"k8s.io/contrib/installer/pkg/packages"
)

type DebianAutoUpgrades struct {
	fi.StructuralUnit
}

func (d *DebianAutoUpgrades) Add(c *fi.BuildContext) {
	if c.OS().IsDebian() {
		c.Add(packages.Installed("unattended-upgrades"))

		c.Add(files.Path("/etc/apt/apt.conf.d/20auto-upgrades").WithContents(buildAutoUpgradesConf()))
	}
}

func buildAutoUpgradesConf() *fi.StringResource {
	return fi.NewStringResource(`
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";

APT::Periodic::AutocleanInterval "7";
`)
}
