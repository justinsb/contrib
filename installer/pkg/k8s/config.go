//package kutil
//
//import (
//	"encoding/json"
//
//	"github.com/golang/glog"
//)
//
//type Configuration struct {
//	//CloudProvider       string
//	//CloudProviderConfig string
//	//
//	//ClusterID string
//	//
//	//MasterInternalIP string
//	//MasterVolume     string
//	//MasterCIDR       string
//	//
//	//InstancePrefix     string
//	//NodeInstancePrefix string
//	//ClusterIPRange     string
//	//AllocateNodeCIDRs  bool
//	//
//	//ServerBinaryTarURL string
//	//SaltTarURL         string
//	//BootstrapURL       string
//	//
//	//Zone         string
//	//KubeUser     string
//	//KubePassword string
//	//
//	//SaltMaster string
//	//MasterName string
//	//
//	//ServiceClusterIPRange        string
//	//EnableClusterMonitoring      string
//	//EnableClusterLogging         bool
//	//EnableNodeLogging            bool
//	//LoggingDestination           string
//	//ElasticsearchLoggingReplicas int
//	//
//	//EnableClusterDNS bool
//	//DNSReplicas      int
//	//DNSServerIP      string
//	//DNSDomain        string
//	//
//	//EnableClusterUI bool
//	//
//	//AdmissionControl string
//	//
//	//KubeletToken   string
//	//KubeProxyToken string
//	//
//	//DockerOptions string
//	//DockerStorage string
//	//
//	//MasterExtraSans []string
//}
//
//func (c *Configuration) AsJson() string {
//	j, err := json.Marshal(c)
//	if err != nil {
//		glog.Fatalf("error marshalling configuration to JSON: %v", err)
//	}
//	return string(j)
//}
