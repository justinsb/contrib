package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"strconv"
	"strings"

	"github.com/golang/glog"
)

type MasterScript struct {
	Config *Configuration
}

var _ DynamicResource = &MasterScript{}

type MinionScript struct {
	Config *Configuration
}

var _ DynamicResource = &MinionScript{}

type ScriptWriter struct {
	buffer bytes.Buffer
}

func (w *ScriptWriter) SetVar(key string, value string) {
	//	buffer.WriteString(fmt.Sprintf("readonly %s='%s'\n", key, value))
	w.buffer.WriteString(fmt.Sprintf("%s='%s'\n", key, value))
}

func (w *ScriptWriter) SetVarInt(key string, value int) {
	w.SetVar(key, strconv.Itoa(value))
}

func (w *ScriptWriter) SetVarBool(key string, value bool) {
	var v string
	if value {
		v = "true"
	} else {
		v = "false"
	}
	w.SetVar(key, v)
}

func (w *ScriptWriter) WriteString(s string) {
	w.buffer.WriteString(s)
}

func (sw *ScriptWriter) WriteTo(w io.Writer) error {
	_, err := sw.buffer.WriteTo(w)
	return err
}

func (s *ScriptWriter) CopyTemplate(key string) {
	templatePath := path.Join(templateDir, key)
	contents, err := ioutil.ReadFile(templatePath)
	if err != nil {
		glog.Fatalf("error reading template (%s): %v", templatePath, err)
	}

	for _, line := range strings.Split(string(contents), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		s.WriteString(line + "\n")
	}
}

func (m *MinionScript) Prefix() string {
	return "minion_script"
}

func (m *MinionScript) Write(w io.Writer) error {
	var s ScriptWriter

	// We send this to the ami as a startup script in the user-data field.  Requires a compatible ami
	s.WriteString("#! /bin/bash\n")

	s.SetVar("SALT_MASTER", m.Config.MasterInternalIP)

	s.SetVar("DOCKER_OPTS", m.Config.DockerOptions)

	s.SetVar("DOCKER_STORAGE", m.Config.DockerStorage)

	s.CopyTemplate("common.sh")
	s.CopyTemplate("format-disks.sh")
	s.CopyTemplate("salt-minion.sh")

	return s.WriteTo(w)
}

func (m *MasterScript) Prefix() string {
	return "master_script"
}

func (m *MasterScript) Write(w io.Writer) error {
	var s ScriptWriter

	// We send this to the ami as a startup script in the user-data field.  Requires a compatible ami
	s.WriteString("#! /bin/bash\n")
	s.WriteString("mkdir -p /var/cache/kubernetes-install\n")
	s.WriteString("cd /var/cache/kubernetes-install\n")

	s.SetVar("SALT_MASTER", m.Config.MasterInternalIP)
	s.SetVar("INSTANCE_PREFIX", m.Config.InstancePrefix)
	s.SetVar("NODE_INSTANCE_PREFIX", m.Config.NodeInstancePrefix)
	s.SetVar("CLUSTER_IP_RANGE", m.Config.ClusterIPRange)
	s.SetVar("ALLOCATE_NODE_CIDRS", m.Config.AllocateNodeCIDRs)
	s.SetVar("SERVER_BINARY_TAR_URL", m.Config.ServerBinaryTarURL)
	s.SetVar("SALT_TAR_URL", m.Config.SaltTarURL)
	s.SetVar("ZONE", m.Config.Zone)
	s.SetVar("KUBE_USER", m.Config.KubeUser)
	s.SetVar("KUBE_PASSWORD", m.Config.KubePassword)

	s.SetVar("SERVICE_CLUSTER_IP_RANGE", m.Config.ServiceClusterIPRange)
	s.SetVar("ENABLE_CLUSTER_MONITORING", m.Config.EnableClusterMonitoring)
	s.SetVar("ENABLE_CLUSTER_LOGGING", m.Config.EnableClusterLogging)
	s.SetVar("ENABLE_NODE_LOGGING", m.Config.EnableNodeLogging)
	s.SetVar("LOGGING_DESTINATION", m.Config.LoggingDestination)
	s.SetVarInt("ELASTICSEARCH_LOGGING_REPLICAS", m.Config.ElasticsearchLoggingReplicas)

	s.SetVarBool("ENABLE_CLUSTER_DNS", m.Config.EnableClusterDNS)
	s.SetVarInt("DNS_REPLICAS", m.Config.DNSReplicas)
	s.SetVar("DNS_SERVER_IP", m.Config.DNSServerIP)
	s.SetVar("DNS_DOMAIN", m.Config.DNSDomain)

	s.SetVarBool("ENABLE_CLUSTER_UI", m.Config.EnableClusterUI)

	s.SetVar("ADMISSION_CONTROL", m.Config.AdmissionControl)
	s.SetVar("MASTER_IP_RANGE", m.Config.MasterIPRange)
	s.SetVar("KUBELET_TOKEN", m.Config.KubeletToken)
	s.SetVar("KUBE_PROXY_TOKEN", m.Config.KubeProxyToken)

	s.SetVar("DOCKER_STORAGE", m.Config.DockerStorage)

	s.SetVar("MASTER_EXTRA_SANS", m.Config.MasterExtraSans)

	s.CopyTemplate("common.sh")
	s.CopyTemplate("format-disks.sh")
	s.CopyTemplate("setup-master-pd.sh")
	s.CopyTemplate("create-dynamic-salt-files.sh")
	s.CopyTemplate("download-release.sh")
	s.CopyTemplate("salt-master.sh")

	return s.WriteTo(w)
}
