package sysctl

import (
	"fmt"
	"os/exec"
	"path"

	"github.com/golang/glog"
	"github.com/kubernetes/contrib/installer/pkg/fi"
	"github.com/kubernetes/contrib/installer/pkg/files"
)

type Sysctl struct {
	Name  string
	Value string
}

func Set(name string, value string) *Sysctl {
	return &Sysctl{
		Name:  name,
		Value: value,
	}
}

func (s *Sysctl) buildSysctlFile() (string, error) {
	return fmt.Sprintf("%s = %s", s.Name, s.Value), nil
}

func (s *Sysctl) Configure(c *fi.Context) error {
	sysctlFile := files.New()
	sysctlFile.Path = path.Join("/etc/sysctl.d", "99-"+s.Name+".conf")
	sysctlFile.Contents = s.buildSysctlFile
	err := sysctlFile.Configure(c)
	if err != nil {
		return err
	}

	glog.V(2).Infof("Updating sysctl setting for %q", s.Name)
	cmd := exec.Command("sysctl", "-p", sysctlFile.Path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running sysctl for  %q: %v: %s", s.Name, err, string(output))
	}

	return nil
}
