package packages

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/golang/glog"
	"github.com/kubernetes/contrib/installer/pkg/fi"
)

const stateKey = "packages"

func Installed(name string) *Package {
	return &Package{
		Name: name,
	}
}

type packageState struct {
	installed map[string]bool
}

func (s *packageState) isInstalled(name string) bool {
	return s.installed[name]
}

func (s *packageState) markInstalled(name string) {
	s.installed[name] = true
}

func findInstalledPackages() (*packageState, error) {
	glog.V(2).Infof("Listing installed packages")
	cmd := exec.Command("dpkg-query", "-f", "${binary:Package}\\n", "-W")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error listing installed packages: %v: %s", err, string(output))
	}
	installed := make(map[string]bool)
	for _, line := range strings.Split(string(output), "\n") {
		installed[line] = true
	}
	return &packageState{installed: installed}, nil
}

type Package struct {
	Name string
}

func (p *Package) Configure(c *fi.RunContext) error {
	state, err := c.GetState(stateKey, func() (interface{}, error) { return findInstalledPackages() })
	if err != nil {
		return err
	}
	packageState := state.(*packageState)
	if packageState.isInstalled(p.Name) {
		glog.V(2).Infof("Package already installed: %q", p.Name)
		return nil
	}

	glog.V(2).Infof("Installing package %q", p.Name)
	cmd := exec.Command("apt-get", "install", "--yes", p.Name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error installing package %q: %v: %s", p.Name, err, string(output))
	}

	packageState.markInstalled(p.Name)

	return nil
}
