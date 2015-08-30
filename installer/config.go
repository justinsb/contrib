package main

type Configuration struct {
	ClusterID          string
	MasterInternalIP   string
	InstancePrefix     string
	NodeInstancePrefix string
	ClusterIPRange     string
	AllocateNodeCIDRs  bool
	ServerBinaryTarURL string
	SaltTarURL         string
	Zone               string
	KubeUser           string
	KubePassword       string

	SaltMaster string
	MasterName string

	ServiceClusterIPRange        string
	EnableClusterMonitoring      string
	EnableClusterLogging         bool
	EnableNodeLogging            bool
	LoggingDestination           string
	ElasticsearchLoggingReplicas int

	EnableClusterDNS bool
	DNSReplicas      int
	DNSServerIP      string
	DNSDomain        string

	EnableClusterUI bool

	AdmissionControl string

	MasterIPRange string

	KubeletToken   string
	KubeProxyToken string

	DockerOptions string
	DockerStorage string

	MasterExtraSans []string
}
