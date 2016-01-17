package sysctl

import (
	"fmt"
	"os/exec"
	"path"
	"strings"

	"github.com/golang/glog"
	"github.com/kubernetes/contrib/installer/pkg/fi"
	"github.com/kubernetes/contrib/installer/pkg/files"
)

type Sysctl struct {
	fi.StructuralUnit
	fi.SystemUnit

	Name  string
	Value string
}

func Set(name string, value string) *Sysctl {
	return &Sysctl{
		Name:  name,
		Value: value,
	}
}

func (s *Sysctl) buildSysctlFile() fi.Resource {
	return fi.NewStringResource(fmt.Sprintf("%s = %s", s.Name, s.Value))
}

func (s *Sysctl) Add(c *fi.BuildContext) {
	sysctlFile := files.New()
	sysctlFile.Path = path.Join("/etc/sysctl.d", "99-"+s.Name+".conf")
	sysctlFile.Contents = s.buildSysctlFile()
	c.Add(sysctlFile)

	c.Add(&applySysctl{Name: s.Name, Value: s.Value})
}

type applySysctl struct {
	fi.SystemUnit

	Name  string
	Value string
}

func (s *applySysctl) Run(c *fi.RunContext) error {
	existingValue := ""
	{
		glog.V(2).Infof("Reading sysctl setting for %q", s.Name)
		cmd := exec.Command("sysctl", "-n", s.Name)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error reading sysctl for  %q: %v: %s", s.Name, err, string(output))
		}
		existingValue = strings.TrimSpace(string(output))
	}

	if existingValue == s.Value {
		glog.V(2).Infof("sysctl value already in place: %s=%s", s.Name, s.Value)
		return nil
	}

	if c.IsConfigure() {
		glog.V(2).Infof("Updating sysctl setting for %q", s.Name)
		cmd := exec.Command("sysctl", "-w", s.Name+"="+s.Value)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error writing sysctl for  %q: %v: %s", s.Name, err, string(output))
		}
		return nil
	} else if c.IsValidate() {
		c.MarkDirty()
		return nil
	} else {
		panic("Unhandled RunMode")
	}
}
