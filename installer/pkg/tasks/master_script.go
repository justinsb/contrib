package tasks

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/kubernetes/contrib/installer/pkg/config"
)

type MasterScript struct {
	Config *config.Configuration
}

var _ DynamicResource = &MasterScript{}

type MinionScript struct {
	Config *config.Configuration
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

func (s *ScriptWriter) CopyTemplate(key string, replacements map[string]string) {
	templatePath := path.Join(templateDir, key)
	contents, err := ioutil.ReadFile(templatePath)
	if err != nil {
		glog.Fatalf("error reading template (%s): %v", templatePath, err)
	}

	for _, line := range strings.Split(string(contents), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Hack to get under 16KB limit
		reString := regexp.QuoteMeta("'$(echo \"$") + "(\\w*)" + regexp.QuoteMeta("\" | sed -e \"s/'/''/g\")'")
		re, err := regexp.Compile(reString)
		if err != nil {
			glog.Fatalf("error compiling regex (%q): %v", reString, err)
		}
		if re.MatchString(line) {
			matches := re.FindStringSubmatch(line)
			key := matches[1]
			v, found := replacements[key]
			if found {
				newLine := re.ReplaceAllString(line, "'"+v+"'")
				glog.V(2).Infof("Replace line %q with %q", line, newLine)
				line = newLine
			} else {
				glog.V(2).Infof("key not found: %q", key)
			}
		}

		s.WriteString(line + "\n")
	}
}

func (s *ScriptWriter) WriteHereDoc(p string, contents string) {
	s.WriteString("cat << E_O_F > " + p + "\n")
	for _, line := range strings.Split(contents, "\n") {
		// TODO: Escaping?
		s.WriteString(line + "\n")
	}
	s.WriteString("E_O_F")
}

func (m *MinionScript) Prefix() string {
	return "minion_script"
}

func (m *MinionScript) Write(w io.Writer) error {
	var s ScriptWriter

	// We send this to the ami as a startup script in the user-data field.  Requires a compatible ami
	s.WriteString("#! /bin/bash\n")

	s.SetVar("SALT_MASTER", m.Config.SaltMaster)
	s.SetVar("DOCKER_OPTS", m.Config.DockerOptions)
	s.SetVar("DOCKER_STORAGE", m.Config.DockerStorage)

	replacements := make(map[string]string)
	s.CopyTemplate("common.sh", replacements)
	s.CopyTemplate("format-disks.sh", replacements)
	s.CopyTemplate("salt-minion.sh", replacements)

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

	replacements := make(map[string]string)

	s.SetVar("SALT_MASTER", m.Config.MasterInternalIP)
	s.SetVar("INSTANCE_PREFIX", m.Config.InstancePrefix)
	replacements["INSTANCE_PREFIX"] = m.Config.InstancePrefix
	s.SetVar("NODE_INSTANCE_PREFIX", m.Config.NodeInstancePrefix)
	replacements["NODE_INSTANCE_PREFIX"] = m.Config.NodeInstancePrefix
	s.SetVar("CLUSTER_IP_RANGE", m.Config.ClusterIPRange)
	replacements["CLUSTER_IP_RANGE"] = m.Config.ClusterIPRange
	s.SetVarBool("ALLOCATE_NODE_CIDRS", m.Config.AllocateNodeCIDRs)
	replacements["ALLOCATE_NODE_CIDRS"] = strconv.FormatBool(m.Config.AllocateNodeCIDRs)

	s.SetVar("SERVER_BINARY_TAR_URL", m.Config.ServerBinaryTarURL)
	s.SetVar("SALT_TAR_URL", m.Config.SaltTarURL)

	s.SetVar("ZONE", m.Config.Zone)
	s.SetVar("KUBE_USER", m.Config.KubeUser)
	s.SetVar("KUBE_PASSWORD", m.Config.KubePassword)

	s.SetVar("SERVICE_CLUSTER_IP_RANGE", m.Config.ServiceClusterIPRange)
	replacements["SERVICE_CLUSTER_IP_RANGE"] = m.Config.ServiceClusterIPRange
	s.SetVar("ENABLE_CLUSTER_MONITORING", m.Config.EnableClusterMonitoring)
	replacements["ENABLE_CLUSTER_MONITORING"] = m.Config.EnableClusterMonitoring
	s.SetVarBool("ENABLE_CLUSTER_LOGGING", m.Config.EnableClusterLogging)
	replacements["ENABLE_CLUSTER_LOGGING"] = strconv.FormatBool(m.Config.EnableClusterLogging)
	s.SetVarBool("ENABLE_NODE_LOGGING", m.Config.EnableNodeLogging)
	replacements["ENABLE_NODE_LOGGING"] = strconv.FormatBool(m.Config.EnableNodeLogging)
	s.SetVar("LOGGING_DESTINATION", m.Config.LoggingDestination)
	replacements["LOGGING_DESTINATION"] = m.Config.LoggingDestination
	s.SetVarInt("ELASTICSEARCH_LOGGING_REPLICAS", m.Config.ElasticsearchLoggingReplicas)
	replacements["ELASTICSEARCH_LOGGING_REPLICAS"] = strconv.Itoa(m.Config.ElasticsearchLoggingReplicas)

	s.SetVarBool("ENABLE_CLUSTER_DNS", m.Config.EnableClusterDNS)
	replacements["ENABLE_CLUSTER_DNS"] = strconv.FormatBool(m.Config.EnableClusterDNS)
	s.SetVarInt("DNS_REPLICAS", m.Config.DNSReplicas)
	replacements["DNS_REPLICAS"] = strconv.Itoa(m.Config.DNSReplicas)
	s.SetVar("DNS_SERVER_IP", m.Config.DNSServerIP)
	replacements["DNS_SERVER_IP"] = m.Config.DNSServerIP
	s.SetVar("DNS_DOMAIN", m.Config.DNSDomain)
	replacements["DNS_DOMAIN"] = m.Config.DNSDomain

	s.SetVarBool("ENABLE_CLUSTER_UI", m.Config.EnableClusterUI)
	replacements["ENABLE_CLUSTER_UI"] = strconv.FormatBool(m.Config.EnableClusterUI)

	s.SetVar("ADMISSION_CONTROL", m.Config.AdmissionControl)
	replacements["ADMISSION_CONTROL"] = m.Config.AdmissionControl
	s.SetVar("MASTER_IP_RANGE", m.Config.MasterCIDR)
	s.SetVar("KUBELET_TOKEN", m.Config.KubeletToken)
	s.SetVar("KUBE_PROXY_TOKEN", m.Config.KubeProxyToken)

	s.SetVar("DOCKER_STORAGE", m.Config.DockerStorage)

	s.SetVar("MASTER_EXTRA_SANS", strings.Join(m.Config.MasterExtraSans, ","))

	s.CopyTemplate("common.sh", replacements)

	s.WriteString("mkdir -p /etc/kubernetes\n")
	s.WriteString("download-or-bust \"" + m.Config.BootstrapURL + "\"\n")
	s.WriteHereDoc("/etc/kubernetes/config.json", m.Config.AsJson())

	s.CopyTemplate("format-disks.sh", replacements)
	s.CopyTemplate("setup-master-pd.sh", replacements)
	s.CopyTemplate("create-dynamic-salt-files.sh", replacements)
	s.CopyTemplate("download-release.sh", replacements)
	s.CopyTemplate("salt-master.sh", replacements)

	return s.WriteTo(w)
}
