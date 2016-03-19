package tasks

import (
	"fmt"
	"io"

	"k8s.io/contrib/installer/pkg/fi"
	"gopkg.in/yaml.v2"
)

type MasterScript struct {
	fi.SimpleUnit

	Config *K8s
}

var _ DynamicResource = &MasterScript{}

type NodeScript struct {
	fi.SimpleUnit

	Config *K8s
}

var _ DynamicResource = &NodeScript{}

func (m *NodeScript) Prefix() string {
	return "minion_script"
}

func writeScript(k*K8s, isMaster bool, w io.Writer) error {
	var s fi.ScriptWriter

	data := k.BuildEnv(true)
	data["AUTO_UPGRADE"] = true
	// TODO: get rid of these exceptions / harmonize with common or GCE
	data["DOCKER_STORAGE"] = k.DockerStorage
	data["API_SERVERS"] = k.MasterInternalIP

	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling env to yaml: %v", err)
	}

	// We send this to the ami as a startup script in the user-data field.  Requires a compatible ami
	s.WriteString("#! /bin/bash\n")
	s.WriteString("mkdir -p /var/cache/kubernetes-install\n")
	s.WriteString("cd /var/cache/kubernetes-install\n")

	s.WriteHereDoc("kube_env.yaml", string(yamlData))

	s.WriteString("wget -O bootstrap " + k.BootstrapScriptURL)
	s.WriteString("chmod +x bootstrap")
	s.WriteString("mkdir -p /etc/kubernetes")
	s.WriteString("mv kube_env.yaml /etc/kubernetes")
	s.WriteString("mv bootstrap /etc/kubernetes/")

	s.WriteString("cat > /etc/rc.local << EOF_RC_LOCAL")
	s.WriteString("#!/bin/sh -e")
	// We want to be sure that we don't pass an argument to bootstrap
	s.WriteString("/etc/kubernetes/bootstrap")
	s.WriteString("exit 0")
	s.WriteString("EOF_RC_LOCAL")
	s.WriteString("/etc/kubernetes/bootstrap")

	return s.WriteTo(w)
}

func (m *NodeScript) WriteTo(w io.Writer) error {
	isMaster := false
	return writeScript(m.Config, isMaster, w)
}

func (m *MasterScript) Prefix() string {
	return "master_script"
}

func (m *MasterScript) WriteTo(w io.Writer) error {
	isMaster := true
	return writeScript(m.Config, isMaster, w)
}