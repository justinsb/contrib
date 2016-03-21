package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/golang/glog"
	"k8s.io/contrib/installer/kutil/pkg/kutil"
	"path"
	"k8s.io/contrib/installer/pkg/tasks"
	"strings"
	"strconv"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"encoding/json"
)

type ExportClusterCmd struct {
	Master      string
	Node        string
	SSHIdentity string
	DestDir     string
}

var exportClusterCmd ExportClusterCmd

func init() {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "export cluster configuration",
		Long: `Connects to your master server over SSH, and exports the configuration from the settings.`,
		Run: func(cmd *cobra.Command, args[]string) {
			err := exportClusterCmd.Run()
			if err != nil {
				glog.Exitf("%v", err)
			}
		},
	}

	exportCmd.AddCommand(cmd)

	cmd.Flags().StringVarP(&exportClusterCmd.Master, "master", "m", "", "Master IP address or hostname")
	cmd.Flags().StringVarP(&exportClusterCmd.Node, "node", "n", "", "Node IP address or hostname")
	cmd.Flags().StringVarP(&exportClusterCmd.SSHIdentity, "i", "i", "", "SSH private key")
	cmd.Flags().StringVarP(&exportClusterCmd.DestDir, "dest", "d", "", "Destination directory")
}

func (c*ExportClusterCmd) Run() error {
	if c.Master == "" {
		return fmt.Errorf("--master must be specified")
	}
	if c.Node == "" {
		return fmt.Errorf("--node must be specified")
	}
	if c.DestDir == "" {
		return fmt.Errorf("--dest must be specified")
	}
	fmt.Printf("Connecting to master on %s\n", c.Master)

	master := &kutil.NodeSSH{
		IP: c.Master,
	}
	err := master.AddSSHIdentity(c.SSHIdentity)
	if err != nil {
		return err
	}

	conf, err := master.ReadConfiguration()
	if err != nil {
		return err
	}

	fmt.Printf("Connecting to node on %s\n", c.Node)

	node := &kutil.NodeSSH{
		IP: c.Master,
	}
	err = node.AddSSHIdentity(c.SSHIdentity)
	if err != nil {
		return err
	}

	instancePrefix := conf.Settings["INSTANCE_PREFIX"]
	if instancePrefix == "" {
		return fmt.Errorf("cannot determine INSTANCE_PREFIX")
	}

	k8s := &tasks.K8s{}
	k8s.CloudProvider = "aws"
	k8s.ClusterID = instancePrefix
	k8s.MasterInstanceType, err = master.InstanceType()
	if err != nil {
		return fmt.Errorf("cannot determine master instance type: %v", err)
	}
	k8s.NodeInstanceType, err = node.InstanceType()
	if err != nil {
		return fmt.Errorf("cannot determine node instance type: %v", err)
	}

	// We want to upgrade!
	// k8s.ImageId = ""

	k8s.ClusterIPRange = conf.Settings["CLUSTER_IP_RANGE"]
	k8s.AllocateNodeCIDRs = parseBool(conf.Settings["ALLOCATE_NODE_CIDRS"])
	k8s.Zone = conf.Settings["ZONE"]
	k8s.KubeUser = conf.Settings["KUBE_USER"]
	k8s.KubePassword = conf.Settings["KUBE_PASSWORD"]
	k8s.ServiceClusterIPRange = conf.Settings["SERVICE_CLUSTER_IP_RANGE"]
	k8s.EnableClusterMonitoring = conf.Settings["ENABLE_CLUSTER_MONITORING"]
	k8s.EnableClusterLogging = parseBool(conf.Settings["ENABLE_CLUSTER_LOGGING"])
	k8s.EnableNodeLogging = parseBool(conf.Settings["ENABLE_NODE_LOGGING"])
	k8s.LoggingDestination = conf.Settings["LOGGING_DESTINATION"]
	k8s.ElasticsearchLoggingReplicas, err = parseInt(conf.Settings["ELASTICSEARCH_LOGGING_REPLICAS"])
	if err != nil {
		return fmt.Errorf("cannot parse ELASTICSEARCH_LOGGING_REPLICAS=%q: %v", conf.Settings["ELASTICSEARCH_LOGGING_REPLICAS"], err)
	}
	k8s.EnableClusterDNS = parseBool(conf.Settings["ENABLE_CLUSTER_DNS"])
	k8s.EnableClusterUI = parseBool(conf.Settings["ENABLE_CLUSTER_UI"])
	k8s.DNSReplicas, err = parseInt(conf.Settings["DNS_REPLICAS"])
	if err != nil {
		return fmt.Errorf("cannot parse DNS_REPLICAS=%q: %v", conf.Settings["DNS_REPLICAS"], err)
	}
	k8s.DNSServerIP = conf.Settings["DNS_SERVER_IP"]
	k8s.DNSDomain = conf.Settings["DNS_DOMAIN"]
	k8s.AdmissionControl = conf.Settings["ADMISSION_CONTROL"]
	k8s.MasterIPRange = conf.Settings["MASTER_IP_RANGE"]
	k8s.DNSServerIP = conf.Settings["DNS_SERVER_IP"]
	k8s.KubeletToken = conf.Settings["KUBELET_TOKEN"]
	k8s.KubeProxyToken = conf.Settings["KUBE_PROXY_TOKEN"]
	k8s.DockerStorage = conf.Settings["DOCKER_STORAGE"]
	//k8s.MasterExtraSans = conf.Settings["MASTER_EXTRA_SANS"] // Not user set
	k8s.NodeCount, err = parseInt(conf.Settings["NUM_MINIONS"])
	if err != nil {
		return fmt.Errorf("cannot parse NUM_MINIONS=%q: %v", conf.Settings["NUM_MINIONS"], err)
	}

	if conf.Version == "1.1" {
		// If users went with defaults on some things, clear them out so they get the new defaults
		if k8s.AdmissionControl == "NamespaceLifecycle,LimitRanger,SecurityContextDeny,ServiceAccount,ResourceQuota" {
			// More admission controllers in 1.2
			k8s.AdmissionControl = ""
		}
		if k8s.MasterInstanceType == "t2.micro" {
			// Different defaults in 1.2
			k8s.MasterInstanceType = ""
		}
		if k8s.NodeInstanceType == "t2.micro" {
			// Encourage users to pick something better...
			k8s.NodeInstanceType = ""
		}
	}


	//b.Context = "aws_" + instancePrefix

	caCertPath := path.Join(c.DestDir, "pki/ca.crt")
	err = downloadFile(master, "/srv/kubernetes/ca.crt", caCertPath)
	if err != nil {
		return err
	}

	kubecfgCertPath := path.Join(c.DestDir, "pki/issued/cn=kubernetes-master.crt")
	err = downloadFile(master, "/srv/kubernetes/kubecfg.crt", kubecfgCertPath)
	if err != nil {
		return err
	}
	kubecfgKeyPath := path.Join(c.DestDir, "pki/private/cn=kubernetes-master.key")
	err = downloadFile(master, "/srv/kubernetes/kubecfg.key", kubecfgKeyPath)
	if err != nil {
		return err
	}

	kubeletCertPath := path.Join(c.DestDir, "pki/issued/cn=kubelet.crt")
	err = downloadFile(node, "/var/run/kubernetes/kubelet.crt", kubeletCertPath)
	if err != nil {
		return err
	}
	kubeletKeyPath := path.Join(c.DestDir, "pki/private/cn=kubelet.key")
	err = downloadFile(node, "/var/run/kubernetes/kubelet.key", kubeletKeyPath)
	if err != nil {
		return err
	}

	confPath := path.Join(c.DestDir, "kubernetes.yaml")
	err = writeConf(confPath, k8s)
	if err != nil {
		return err
	}

	return nil
}

func parseBool(s string) bool {
	s = strings.ToLower(s)
	if s == "true" || s == "1" || s == "y" || s == "yes" || s == "t" {
		return true
	}
	return false
}

func parseInt(s string) (int, error) {
	if s == "" {
		return 0, nil
	}

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}

	return int(n), nil
}

func writeConf(p string, k8s *tasks.K8s) (error) {
	jsonBytes, err := json.Marshal(k8s)
	if err != nil {
		return fmt.Errorf("error serializing configuration (json write phase): %v", err)
	}

	var confObj interface{}
	err = yaml.Unmarshal(jsonBytes, &confObj)
	if err != nil {
		return fmt.Errorf("error serializing configuration (yaml read phase): %v", err)
	}


	m := confObj.(map[interface{}]interface{})

	for k, v := range m {
		if v == nil {
			delete(m, k)
		}
		s, ok := v.(string)
		if ok && s == "" {
			delete(m, k)
		}
		//glog.Infof("%v=%v", k, v)
	}

	yaml, err := yaml.Marshal(confObj)
	if err != nil {
		return fmt.Errorf("error serializing configuration (yaml write phase): %v", err)
	}

	err = ioutil.WriteFile(p, yaml, 0600)
	if err != nil {
		return fmt.Errorf("error writing configuration to file %q: %v", p, err)
	}

	return nil
}