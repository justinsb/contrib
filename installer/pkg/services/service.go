package services

import (
	"bytes"
	"fmt"
	"math"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/kubernetes/contrib/installer/pkg/fi"
	"github.com/kubernetes/contrib/installer/pkg/files"
)

func Running(name string) *Service {
	return &Service{
		Name: name,
	}
}

type Service struct {
	Name string

	Description   string
	Documentation string
	After         []string
	Requires      []string

	Environment map[string]string
	Exec        string
	MountFlags  string

	Limits Limits

	// If RunOnce is true, service will not have auto-restart stuff specified
	RunOnce bool
}

type Limits struct {
	Files     uint64
	Processes uint64
	CoreDump  uint64
}

func systemctlDaemonReload() error {
	glog.V(2).Infof("Doing systemd daemon-reload")
	cmd := exec.Command("systemctl", "daemon-reload")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error during systemd daemon-reload: %v: %s", err, string(output))
	}
	return nil
}

func (s *Service) buildEnvFile() (string, error) {
	var b bytes.Buffer
	for k, v := range s.Environment {
		b.Write([]byte(k))
		b.Write([]byte("=\""))
		// TODO: Escaping
		b.Write([]byte(v))
		b.Write([]byte("\"\n"))
	}
	return string(b.Bytes()), nil
}

type IniFile struct {
	b   bytes.Buffer
	err error
}

func (i *IniFile) Bytes() []byte {
	return i.b.Bytes()
}

func (i *IniFile) Error() error {
	return i.err
}

func (i *IniFile) StartSection(name string) {
	fmt.Fprintf(&i.b, "[%s]\n", name)
}

func (i *IniFile) WriteKey(key string, value string) {
	// TODO: Escaping
	fmt.Fprintf(&i.b, "%s=%s\n", key, value)
}

func (s *Service) buildUnitFile() (string, error) {
	var i IniFile
	i.StartSection("Unit")
	if s.Description != "" {
		i.WriteKey("Description", s.Description)
	}
	if s.Documentation != "" {
		i.WriteKey("Documentation", s.Documentation)
	}
	if len(s.After) != 0 {
		i.WriteKey("After", strings.Join(s.After, " "))
	}
	if len(s.Requires) != 0 {
		i.WriteKey("Requires", strings.Join(s.Requires, " "))
	}

	i.StartSection("Service")
	if s.Environment != nil {
		i.WriteKey("EnvironmentFile", s.envFilePath())
	}
	if s.Exec != "" {
		i.WriteKey("ExecStart", s.Exec)
	}
	if s.MountFlags != "" {
		i.WriteKey("MountFlags", s.MountFlags)
	}

	if s.Limits.Files != 0 {
		i.WriteKey("LimitNOFILE", strconv.FormatUint(s.Limits.Files, 10))
	}
	if s.Limits.Processes != 0 {
		i.WriteKey("LimitNPROC", strconv.FormatUint(s.Limits.Processes, 10))
	}
	if s.Limits.CoreDump != 0 {
		if s.Limits.CoreDump == math.MaxUint64 {
			i.WriteKey("LimitCORE", "infinity")
		} else {
			i.WriteKey("LimitCORE", strconv.FormatUint(s.Limits.CoreDump, 10))
		}
	}

	if !s.RunOnce {
		i.WriteKey("Restart", "always")
		i.WriteKey("RestartSec", "2s")
		i.WriteKey("StartLimitInterval", "0")
	}

	i.StartSection("Install")
	i.WriteKey("WantedBy", "multi-user.target")

	return string(i.Bytes()), i.Error()
}

func (s *Service) envFilePath() string {
	return path.Join("/etc/sysconfig", s.Name)
}

func (s *Service) Configure(context *fi.Context) error {
	if s.Exec != "" {
		// TODO: Expose tree structure
		unitfile := files.New()
		unitfile.Path = path.Join("/lib/systemd/system", s.Name+".service")
		unitfile.Contents = s.buildUnitFile
		err := unitfile.Configure(context)
		if err != nil {
			return err
		}

		if s.Environment != nil {
			envfile := files.New()
			envfile.Path = s.envFilePath()
			envfile.Contents = s.buildEnvFile
			err := envfile.Configure(context)
			if err != nil {
				return err
			}
		}

		// TODO: Only if dirty
		err = systemctlDaemonReload()
		if err != nil {
			return err
		}
	}

	// TODO: Only if not running

	glog.V(2).Infof("Start service %q", s.Name)
	cmd := exec.Command("systemctl", "start", s.Name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error starting service %q: %v: %s", s.Name, err, string(output))
	}
	return nil
}
