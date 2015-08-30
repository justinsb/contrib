package main

type Configuration struct {
MasterInternalIP   string
	InstancePrefix     string
	NodeInstancePrefix string
	ClusterIPRange     string
	AllocateNodeCIDRs  string
	ServerBinaryTarURL string
	SaltTarURL         string
	Zone               string
	KubeUser           string
	KubePassword       string

	ServiceClusterIPRange        string
	EnableClusterMonitoring      string
	EnableClusterLogging         string
	EnableNodeLogging            string
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

	MasterExtraSans string
}
