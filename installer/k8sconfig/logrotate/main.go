package logrotate

import (
	"path"

	"github.com/kubernetes/contrib/installer/pkg/fi"
	"github.com/kubernetes/contrib/installer/pkg/files"
	"github.com/kubernetes/contrib/installer/pkg/packages"
)

type LogRotate struct {
	fi.StructuralUnit
}

func (l *LogRotate) Add(c *fi.BuildContext) {
	c.Add(packages.Installed("logrotate"))

	files := []string{"kube-scheduler", "kube-proxy", "kubelet", "kube-apiserver", "kube-controller-manager", "kube-addons", "docker"}
	for _, f := range files {
		c.Add(&LogRotateFile{Key: f})
	}

	c.Add(&LogRotateFile{Key: "docker-containers", LogPath: "/var/lib/docker/containers/*/*-json.log"})

	c.Add(buildLogrotateCron())
}

func buildLogrotateCron() *files.File {
	script := `#!/bin/sh
logrotate /etc/logrotate.conf`

	return files.Path("/etc/cron.hourly/logrotate").WithContents(fi.NewStringResource(script)).WithMode(0755)
}

type LogRotateFile struct {
	Key     string
	LogPath string
}

func (l *LogRotateFile) buildConf() fi.Resource {
	var sb fi.StringBuilder

	logPath := l.LogPath
	if logPath == "" {
		logPath = path.Join("/var/log", l.Key+".log")
	}
	sb.Append(logPath + " {\n")
	sb.Append("\trotate 5\n")
	sb.Append("\tcopytruncate\n")
	sb.Append("\tmissingok\n")
	sb.Append("\tnotifempty\n")
	sb.Append("\tcompress\n")
	sb.Append("\tmaxsize 100M\n")
	sb.Append("\tdaily\n")
	sb.Append("\tcreate 0644 root root\n")
	sb.Append("}\n")

	return sb.AsResource()
}

func (l *LogRotateFile) Configure(c *fi.RunContext) error {
	confPath := path.Join("/etc/logrotate.d", l.Key)
	confFile := files.Path(confPath).WithContents(l.buildConf())
	err := confFile.Configure(c)
	if err != nil {
		return err
	}

	return nil
}
