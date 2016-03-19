package tasks

import (
	"fmt"
	"io"

	"k8s.io/contrib/installer/pkg/fi"
	"gopkg.in/yaml.v2"
	"bytes"
)

type MasterScript struct {
	fi.SimpleUnit

	Config   *K8s

	contents string
}

func (s *MasterScript) Key() string {
	return "master-script"
}

var _ fi.Resource = &MasterScript{}

type NodeScript struct {
	fi.SimpleUnit

	Config   *K8s

	contents string
}

var _ fi.Resource = &NodeScript{}

func (s *NodeScript) Key() string {
	return "node-script"
}

//func (m *NodeScript) Prefix() string {
//	return "node_script"
//}

func buildScript(c *fi.RunContext, k *K8s, isMaster bool) (string, error) {
	var bootstrapScriptURL string

	{
		url, _, err := c.Target.PutResource("bootstrap", k.BootstrapScript)
		if err != nil {
			return "", err
		}
		bootstrapScriptURL = url
	}

	data, err := k.BuildEnv(c, isMaster)
	if err != nil {
		return "", err
	}
	data["AUTO_UPGRADE"] = true
	// TODO: get rid of these exceptions / harmonize with common or GCE
	data["DOCKER_STORAGE"] = k.DockerStorage
	data["API_SERVERS"] = k.MasterInternalIP

	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("error marshaling env to yaml: %v", err)
	}

	// We send this to the ami as a startup script in the user-data field.  Requires a compatible ami
	var s fi.ScriptWriter
	s.WriteString("#! /bin/bash\n")
	s.WriteString("mkdir -p /var/cache/kubernetes-install\n")
	s.WriteString("cd /var/cache/kubernetes-install\n")

	s.WriteHereDoc("kube_env.yaml", string(yamlData))

	s.WriteString("wget -O bootstrap " + bootstrapScriptURL)
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

	return s.AsString(), nil
}

func (m*MasterScript) Run(c *fi.RunContext) error {
	isMaster := true
	contents, err := buildScript(c, m.Config, isMaster)
	if err != nil {
		return err
	}
	m.contents = contents
	return nil
}

func (m *MasterScript) Open() (io.ReadSeeker, error) {
	if m.contents == "" {
		panic("executed out of sequence")
	}
	return bytes.NewReader([]byte(m.contents)), nil
}

func (m*NodeScript) Run(c *fi.RunContext) error {
	isMaster := false
	contents, err := buildScript(c, m.Config, isMaster)
	if err != nil {
		return err
	}
	m.contents = contents
	return nil
}

func (m *NodeScript) Open() (io.ReadSeeker, error) {
	if m.contents == "" {
		panic("executed out of sequence")
	}
	return bytes.NewReader([]byte(m.contents)), nil
}

//func (m *MasterScript) Prefix() string {
//	return "master_script"
//}
