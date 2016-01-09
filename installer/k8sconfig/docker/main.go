//go:generate esc -o res.go -pkg docker res
package docker

import (
	"math"

	"github.com/kubernetes/contrib/installer/pkg/fi"
	"github.com/kubernetes/contrib/installer/pkg/packages"
	"github.com/kubernetes/contrib/installer/pkg/services"
	"github.com/kubernetes/contrib/installer/pkg/sysctl"
)

func Add(context *fi.Context) {
	context.Add(packages.Installed("bridge-utils"))

	context.Add(sysctl.Set("net.ipv4.ip_forward", "1"))

	d := &packages.Package{
		Name: "lxc-docker-1.7.1",
	}
	context.Add(d)

	args := context.Get("docker_opts")
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
	context.Add(s)
}
