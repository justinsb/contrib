//go:generate esc -o res.go -pkg docker res
package docker

import (
	"math"

	"k8s.io/contrib/installer/pkg/fi"
	"k8s.io/contrib/installer/pkg/packages"
	"k8s.io/contrib/installer/pkg/services"
	"k8s.io/contrib/installer/pkg/sysctl"
)

type Docker struct {
	fi.StructuralUnit

	DockerOpts string
}

func (d *Docker) Add(c *fi.BuildContext) {
	c.Add(packages.Installed("bridge-utils"))

	c.Add(sysctl.Set("net.ipv4.ip_forward", "1"))

	p := &packages.Package{
		Name: "lxc-docker-1.7.1",
	}
	c.Add(p)

	args := d.DockerOpts
	//{% set e2e_opts = '' -%}
	//{% if pillar.get('e2e_storage_test_environment', '').lower() == 'true' -%}
	//{% set e2e_opts = '-s devicemapper' -%}
	//{% endif -%}
	// DOCKER_OPTS = "{{grains_opts}} {{e2e_opts}} --bridge=cbr0 --iptables=false --ip-masq=false --log-level=warn"
	args += " --bridge cbr0"
	args += " --iptables=false"
	args += " --ip-masq=false"
	args += " --log-level=warn"

	command := "/usr/bin/docker -d -H fd:// " + args

	// Note that we have inlined DOCKER_OPTS, and DOCKER_NOFILE doesn't do anything
	s := &services.Service{
		Name:          "docker",
		Description:   "Docker Application Container Engine",
		Documentation: "http://docs.docker.com",
		After:         []string{"network.target", "docker.socket"},
		Requires:      []string{"docker.socket"},

		Exec:       command,
		MountFlags: "slave",
		Limits: services.Limits{
			Files:     1048576,
			Processes: 1048576,
			CoreDump:  math.MaxUint64,
		},
	}
	c.Add(s)
}
