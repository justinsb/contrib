package logrotate

import (
	"path"

	"github.com/kubernetes/contrib/installer/pkg/fi"
	"github.com/kubernetes/contrib/installer/pkg/files"
	"github.com/kubernetes/contrib/installer/pkg/packages"
)

func Add(context *fi.Context) {
	context.Add(packages.Installed("logrotate"))

	files := []string{"kube-scheduler", "kube-proxy", "kubelet", "kube-apiserver", "kube-controller-manager", "kube-addons", "docker"}
	for _, f := range files {
		context.Add(&logRotate{Key: f})
	}

	context.Add(&logRotate{Key: "docker-containers", LogPath: "/var/lib/docker/containers/*/*-json.log"})

	context.Add(buildLogrotateCron())
}

func buildLogrotateCron() *files.File {
	script := `#!/bin/sh
logrotate /etc/logrotate.conf`

	return files.Path("/etc/cron.hourly/logrotate").WithContents(fi.StaticContent(script)).WithMode(0755)
}

type logRotate struct {
	Key     string
	LogPath string
}

func (l *logRotate) buildConf() (string, error) {
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

	return sb.String(), sb.Error()
}

func (l *logRotate) Configure(c *fi.Context) error {
	confPath := path.Join("/etc/logrotate.d", l.Key)
	confFile := files.Path(confPath).WithContents(l.buildConf)
	err := confFile.Configure(c)
	if err != nil {
		return err
	}

	return nil
}
