package logrotate

import (
	"path"

	"k8s.io/contrib/installer/pkg/fi"
	"k8s.io/contrib/installer/pkg/files"
	"k8s.io/contrib/installer/pkg/packages"
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

	c.Add(&LogRotateFile{Key: "docker-containers", MaxSize: "10M", LogPath: "/var/lib/docker/containers/*/*-json.log"})

	c.Add(buildLogrotateCron())
}

func buildLogrotateCron() *files.File {
	script := `#!/bin/sh
logrotate /etc/logrotate.conf
`

	return files.Path("/etc/cron.hourly/logrotate").WithContents(fi.NewStringResource(script)).WithMode(0755)
}

type LogRotateFile struct {
	fi.StructuralUnit

	Key     string
	LogPath string

	MaxSize string
}

func (l *LogRotateFile) Add(c *fi.BuildContext) {
	confPath := path.Join("/etc/logrotate.d", l.Key)
	confFile := files.Path(confPath).WithContents(l.buildConf())
	c.Add(confFile)
}

func (l *LogRotateFile) buildConf() fi.Resource {
	var sb fi.StringBuilder

	logPath := l.LogPath
	if logPath == "" {
		logPath = path.Join("/var/log", l.Key+".log")
	}
	sb.Append(logPath + " {\n")
	sb.Append("    rotate 5\n")
	sb.Append("    copytruncate\n")
	sb.Append("    missingok\n")
	sb.Append("    notifempty\n")
	sb.Append("    compress\n")
	maxSize := l.MaxSize
	if maxSize == "" {
		maxSize = "100M"
	}
	sb.Append("    maxsize " + maxSize + "\n")
	sb.Append("    daily\n")
	sb.Append("    create 0644 root root\n")
	sb.Append("}\n")

	return sb.AsResource()
}
