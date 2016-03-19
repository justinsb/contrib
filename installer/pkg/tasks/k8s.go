package tasks

import (
	"k8s.io/contrib/installer/pkg/fi"
	"github.com/golang/glog"
	"strconv"
	"time"
	"path"
)

type K8s struct {
	fi.StructuralUnit

	S3Region                      string
	S3BucketName                  string

	CloudProvider                 string
	CloudProviderConfig           string

	ClusterID                     string

	MasterInstanceType            string
	NodeInstanceType              string

	ImageID                       string

	MasterInternalIP              string
	// TODO: Just move to master volume?
	MasterVolume                  string
	MasterVolumeSize              int
	MasterVolumeType              string
	MasterCIDR                    string

	NodeCount                     int

	InstancePrefix                string
	NodeInstancePrefix            string
	ClusterIPRange                string
	MasterIPRange                 string
	AllocateNodeCIDRs             bool

	ServerBinaryTar               Resource
	SaltTar                       Resource
	BootstrapScript               Resource

	Zone                          string
	KubeUser                      string
	KubePassword                  string

	SaltMaster                    string
	MasterName                    string

	ServiceClusterIPRange         string
	EnableL7LoadBalancing         bool
	EnableClusterMonitoring       string
	EnableClusterLogging          bool
	EnableNodeLogging             bool
	LoggingDestination            string
	ElasticsearchLoggingReplicas  int

	EnableClusterRegistry         bool
	ClusterRegistryDisk           string
	ClusterRegistryDiskSize       int

	EnableClusterDNS              bool
	DNSReplicas                   int
	DNSServerIP                   string
	DNSDomain                     string

	RuntimeConfig                 string

	CaCert                        string
	KubeletCert                   string
	KubeletKey                    string
	KubeletToken                  string
	KubeProxyToken                string
	BearerToken                   string
	MasterCert                    string
	MasterKey                     string
	KubecfgCert                   string
	KubecfgKey                    string

	KubeletApiserver              string

	EnableManifestURL             bool
	ManifestURL                   string
	ManifestURLHeader             string

	NetworkProvider               string

	HairpinMode                   string

	OpencontrailTag               string
	OpencontrailKubernetesTag     string
	OpencontrailPublicSubnet      string

	KubeImageTag                  string
	KubeDockerRegistry            string
	KubeAddonRegistry             string

	Multizone                     bool

	NonMasqueradeCidr             string

	E2EStorageTestEnvironment     string

	EnableClusterUI               bool

	AdmissionControl              string

	KubeletPort                   int

	KubeApiserverRequestTimeout   int

	TerminatedPodGcThreshold      string

	KubeManifestsTarURL           string
	KubeManifestsTarHash          string

	TestCluster                   string

	DockerOptions                 string
	DockerStorage                 string

	MasterExtraSans               []string

	// TODO: Make struct?
	KubeletTestArgs               string
	KubeletTestLogLevel           string
	DockerTestArgs                string
	DockerTestLogLevel            string
	ApiserverTestArgs             string
	ApiserverTestLogLevel         string
	ControllerManagerTestArgs     string
	ControllerManagerTestLogLevel string
	SchedulerTestArgs             string
	SchedulerTestLogLevel         string
	KubeProxyTestArgs             string
	KubeProxyTestLogLevel         string

	NodeLabels                    string
	OsDistribution                string

	ExtraDockerOpts               string

	ContainerRuntime              string
	RktVersion                    string
	RktPath                       string
	KubernetesConfigureCbr0       string

	EnableCustomMetrics           bool
}

func (k*K8s) BuildEnv(c *fi.RunContext, isMaster bool) (map[string]interface{}, error) {
	y := map[string]interface{}{}
	y["ENV_TIMESTAMP"] = time.Now().UTC().Format("2006-01-02T03:04:05+0000")
	y["INSTANCE_PREFIX"] = k.InstancePrefix
	y["NODE_INSTANCE_PREFIX"] = k.NodeInstancePrefix
	y["CLUSTER_IP_RANGE"] = k.ClusterIPRange

	{
		url, hash, err := c.UploadResource(k.ServerBinaryTar)
		if err != nil {
			return err
		}
		y["SERVER_BINARY_TAR_URL"] = url
		y["SERVER_BINARY_TAR_HASH"] = hash
	}

	{
		url, hash, err := c.UploadResource(k.SaltTar)
		if err != nil {
			return err
		}
		y["SALT_TAR_URL"] = url
		y["SALT_TAR_HASH"] = hash
	}

	y["SERVICE_CLUSTER_IP_RANGE"] = k.ServiceClusterIPRange

	y["KUBERNETES_MASTER_NAME"] = k.MasterName

	y["ALLOCATE_NODE_CIDRS"] = k.AllocateNodeCIDRs

	y["ENABLE_CLUSTER_MONITORING"] = k.EnableClusterMonitoring
	y["ENABLE_L7_LOADBALANCING"] = k.EnableL7LoadBalancing
	y["ENABLE_CLUSTER_LOGGING"] = k.EnableClusterLogging
	y["ENABLE_CLUSTER_UI"] = k.EnableClusterUI
	y["ENABLE_NODE_LOGGING"] = k.EnableNodeLogging
	y["LOGGING_DESTINATION"] = k.LoggingDestination
	y["ELASTICSEARCH_LOGGING_REPLICAS"] = k.ElasticsearchLoggingReplicas
	y["ENABLE_CLUSTER_DNS"] = k.EnableClusterDNS
	y["ENABLE_CLUSTER_REGISTRY"] = k.EnableClusterRegistry
	y["CLUSTER_REGISTRY_DISK"] = k.ClusterRegistryDisk
	y["CLUSTER_REGISTRY_DISK_SIZE"] = k.ClusterRegistryDiskSize
	y["DNS_REPLICAS"] = k.DNSReplicas
	y["DNS_SERVER_IP"] = k.DNSServerIP
	y["DNS_DOMAIN"] = k.DNSDomain

	y["KUBELET_TOKEN"] = k.KubeletToken
	y["KUBE_PROXY_TOKEN"] = k.KubeProxyToken
	y["ADMISSION_CONTROL"] = k.AdmissionControl
	y["MASTER_IP_RANGE"] = k.MasterIPRange
	y["RUNTIME_CONFIG"] = k.RuntimeConfig
	y["CA_CERT"] = k.CaCert
	y["KUBELET_CERT"] = k.KubeletCert
	y["KUBELET_KEY"] = k.KubeletKey
	y["NETWORK_PROVIDER"] = k.NetworkProvider
	y["HAIRPIN_MODE"] = k.HairpinMode
	y["OPENCONTRAIL_TAG"] = k.OpencontrailTag
	y["OPENCONTRAIL_KUBERNETES_TAG"] = k.OpencontrailKubernetesTag
	y["OPENCONTRAIL_PUBLIC_SUBNET"] = k.OpencontrailPublicSubnet
	y["E2E_STORAGE_TEST_ENVIRONMENT"] = k.E2EStorageTestEnvironment
	y["KUBE_IMAGE_TAG"] = k.KubeImageTag
	y["KUBE_DOCKER_REGISTRY"] = k.KubeDockerRegistry
	y["KUBE_ADDON_REGISTRY"] = k.KubeAddonRegistry
	y["MULTIZONE"] = k.Multizone
	y["NON_MASQUERADE_CIDR"] = k.NonMasqueradeCidr

	//: $(yaml-quote ${ENABLE_CLUSTER_MONITORING:-none})
	//: $(yaml-quote ${ENABLE_L7_LOADBALANCING:-none})
	//: $(yaml-quote ${ENABLE_CLUSTER_LOGGING:-false})
	//: $(yaml-quote ${ENABLE_CLUSTER_UI:-false})
	//: $(yaml-quote ${ENABLE_NODE_LOGGING:-false})
	//: $(yaml-quote ${LOGGING_DESTINATION:-})
	//: $(yaml-quote ${ELASTICSEARCH_LOGGING_REPLICAS:-})
	//: $(yaml-quote ${ENABLE_CLUSTER_DNS:-false})
	//: $(yaml-quote ${ENABLE_CLUSTER_REGISTRY:-false})
	//: $(yaml-quote ${CLUSTER_REGISTRY_DISK:-})
	//: $(yaml-quote ${CLUSTER_REGISTRY_DISK_SIZE:-})
	//: $(yaml-quote ${DNS_REPLICAS:-})
	//: $(yaml-quote ${DNS_SERVER_IP:-})
	//: $(yaml-quote ${DNS_DOMAIN:-})
	//: $(yaml-quote ${KUBELET_TOKEN:-})
	//: $(yaml-quote ${KUBE_PROXY_TOKEN:-})
	//: $(yaml-quote ${ADMISSION_CONTROL:-})
	//: $(yaml-quote ${MASTER_IP_RANGE})
	//: $(yaml-quote ${RUNTIME_CONFIG})
	//: $(yaml-quote ${CA_CERT_BASE64:-})
	//: $(yaml-quote ${KUBELET_CERT_BASE64:-})
	//: $(yaml-quote ${KUBELET_KEY_BASE64:-})
	//: $(yaml-quote ${NETWORK_PROVIDER:-})
	//: $(yaml-quote ${HAIRPIN_MODE:-})
	//: $(yaml-quote ${OPENCONTRAIL_TAG:-})
	//: $(yaml-quote ${OPENCONTRAIL_KUBERNETES_TAG:-})
	//: $(yaml-quote ${OPENCONTRAIL_PUBLIC_SUBNET:-})
	//: $(yaml-quote ${E2E_STORAGE_TEST_ENVIRONMENT:-})
	//: $(yaml-quote ${KUBE_IMAGE_TAG:-})
	//: $(yaml-quote ${KUBE_DOCKER_REGISTRY:-})
	//: $(yaml-quote ${KUBE_ADDON_REGISTRY:-})
	//: $(yaml-quote ${MULTIZONE:-})
	//: $(yaml-quote ${NON_MASQUERADE_CIDR:-})

	if k.KubeletPort != 0 {
		y["KUBELET_PORT"] = k.KubeletPort
	}

	if k.KubeApiserverRequestTimeout != 0 {
		y["KUBE_APISERVER_REQUEST_TIMEOUT"] = k.KubeApiserverRequestTimeout
	}

	if k.TerminatedPodGcThreshold != "" {
		y["TERMINATED_POD_GC_THRESHOLD"] = k.TerminatedPodGcThreshold
	}

	if k.OsDistribution == "trusty" {
		y["KUBE_MANIFESTS_TAR_URL"] = k.KubeManifestsTarURL
		y["KUBE_MANIFESTS_TAR_HASH"] = k.KubeManifestsTarHash
	}

	if k.TestCluster != "" {
		y["TEST_CLUSTER"] = k.TestCluster
	}

	if k.KubeletTestArgs != "" {
		y["KUBELET_TEST_ARGS"] = k.KubeletTestArgs
	}

	if k.KubeletTestLogLevel != "" {
		y["KUBELET_TEST_LOG_LEVEL"] = k.KubeletTestLogLevel
	}

	if k.DockerTestLogLevel != "" {
		y["DOCKER_TEST_LOG_LEVEL"] = k.DockerTestLogLevel
	}

	if k.EnableCustomMetrics {
		y["ENABLE_CUSTOM_METRICS"] = k.EnableCustomMetrics
	}

	if isMaster {
		y["KUBERNETES_MASTER"] = true
		y["KUBE_USER"] = k.KubeUser
		y["KUBE_PASSWORD"] = k.KubePassword
		y["KUBE_BEARER_TOKEN"] = k.BearerToken
		y["MASTER_CERT"] = k.MasterCert
		y["MASTER_KEY"] = k.MasterKey
		y["KUBECFG_CERT"] = k.KubecfgCert
		y["KUBECFG_KEY"] = k.KubecfgKey
		y["KUBELET_APISERVER"] = k.KubeletApiserver
		y["ENABLE_MANIFEST_URL"] = k.EnableManifestURL
		y["MANIFEST_URL"] = k.ManifestURL
		y["MANIFEST_URL_HEADER"] = k.ManifestURLHeader
		y["NUM_NODES"] = k.NodeCount

		//	: $(yaml-quote ${KUBE_USER})
		//: $(yaml-quote ${KUBE_PASSWORD})
		//: $(yaml-quote ${KUBE_BEARER_TOKEN})
		//: $(yaml-quote ${MASTER_CERT_BASE64:-})
		//: $(yaml-quote ${MASTER_KEY_BASE64:-})
		//: $(yaml-quote ${KUBECFG_CERT_BASE64:-})
		//: $(yaml-quote ${KUBECFG_KEY_BASE64:-})
		//: $(yaml-quote ${KUBELET_APISERVER:-})
		//: $(yaml-quote ${ENABLE_MANIFEST_URL:-false})
		//: $(yaml-quote ${MANIFEST_URL:-})
		//: $(yaml-quote ${MANIFEST_URL_HEADER:-})
		//: $(yaml-quote ${NUM_NODES})

		if k.ApiserverTestArgs != "" {
			y["APISERVER_TEST_ARGS"] = k.ApiserverTestArgs
		}

		if k.ApiserverTestLogLevel != "" {
			y["APISERVER_TEST_LOG_LEVEL"] = k.ApiserverTestLogLevel
		}

		if k.ControllerManagerTestArgs != "" {
			y["CONTROLLER_MANAGER_TEST_ARGS"] = k.ControllerManagerTestArgs
		}

		if k.ControllerManagerTestLogLevel != "" {
			y["CONTROLLER_MANAGER_TEST_LOG_LEVEL"] = k.ControllerManagerTestLogLevel
		}

		if k.SchedulerTestArgs != "" {
			y["SCHEDULER_TEST_ARGS"] = k.SchedulerTestArgs
		}

		if k.SchedulerTestLogLevel != "" {
			y["SCHEDULER_TEST_LOG_LEVEL"] = k.SchedulerTestLogLevel
		}

	}

	if !isMaster {
		// Node-only vars

		y["KUBERNETES_MASTER"] = false
		y["ZONE"] = k.Zone
		y["EXTRA_DOCKER_OPTS"] = k.ExtraDockerOpts
		y["MANIFEST_URL"] = k.ManifestURL

		if k.KubeProxyTestArgs != "" {
			y["KUBEPROXY_TEST_ARGS"] = k.KubeProxyTestArgs
		}

		if k.KubeProxyTestLogLevel != "" {
			y["KUBEPROXY_TEST_LOG_LEVEL"] = k.KubeProxyTestLogLevel
		}
	}

	if k.NodeLabels != "" {
		y["NODE_LABELS"] = k.NodeLabels
	}

	if k.OsDistribution == "coreos" {
		// CoreOS-only env vars. TODO(yifan): Make them available on other distros.
		y["KUBE_MANIFESTS_TAR_URL"] = k.KubeManifestsTarURL
		y["KUBE_MANIFESTS_TAR_HASH"] = k.KubeManifestsTarHash
		y["KUBERNETES_CONTAINER_RUNTIME"] = k.ContainerRuntime
		y["RKT_VERSION"] = k.RktVersion
		y["RKT_PATH"] = k.RktPath
		y["KUBERNETES_CONFIGURE_CBR0"] = k.KubernetesConfigureCbr0

		//		: $(yaml-quote ${KUBE_MANIFESTS_TAR_URL})
		//: $(yaml-quote ${KUBE_MANIFESTS_TAR_HASH})
		//: $(yaml-quote ${CONTAINER_RUNTIME:-docker})
		//: $(yaml-quote ${RKT_VERSION:-})
		//: $(yaml-quote ${RKT_PATH:-})
		//: $(yaml-quote ${KUBERNETES_CONFIGURE_CBR0:-true})
	}

	return y
}

func (k *K8s) Add(c *fi.BuildContext) {
	clusterID := k.ClusterID
	if clusterID == "" {
		glog.Exit("cluster-id is required")
	}

	masterInstanceType := k.MasterInstanceType
	if masterInstanceType == "" {
		masterInstanceType = "m3.medium"
	}
	nodeInstanceType := k.NodeInstanceType
	if nodeInstanceType == "" {
		nodeInstanceType = "m3.medium"
	}

	masterInternalIP := k.MasterInternalIP
	if masterInternalIP == "" {
		masterInstanceType = "172.20.0.9"
	}

	az := k.Zone
	if len(az) <= 2 {
		glog.Exit("Invalid AZ: ", az)
	}
	region := az[:len(az) - 1]


	//s3BucketName := k.S3BucketName
	//if k.S3BucketName == "" {
	//	// TODO: Implement the generation logic
	//	glog.Exit("s3-bucket is required (for now!)")
	//}

	//s3Region := k.S3Region
	//if s3Region == "" {
	//	s3Region = region
	//}

	nodeCount := k.NodeCount
	if nodeCount == 0 {
		nodeCount = 2
	}

	imageID := k.ImageID
	if imageID == "" {
		distro := &DistroVivid{}
		imageID = distro.GetImageID(region)

		if imageID == "" {
			glog.Fatal("ImageID could not be determined")
		}
	}

	masterVolumeSize := k.MasterVolumeSize
	if masterVolumeSize == 0 {
		masterVolumeSize = 20
	}

	masterVolumeType := k.MasterVolumeType
	if masterVolumeType == "" {
		masterVolumeType = "gp2"
	}

	//s3Bucket := &S3Bucket{
	//	Name:         String(s3BucketName),
	//	Region: String(s3Region),
	//}
	//c.Add(s3Bucket)
	//
	//s3KubernetesFile := &S3File{
	//	Bucket: s3Bucket,
	//	Key:    String("devel/kubernetes-server-linux-amd64.tar.gz"),
	//	Source: findKubernetesTarGz(),
	//	Public: Bool(true),
	//}
	//c.Add(s3KubernetesFile)
	//
	//s3SaltFile := &S3File{
	//	Bucket: s3Bucket,
	//	Key:    String("devel/kubernetes-salt.tar.gz"),
	//	Source: findSaltTarGz(),
	//	Public: Bool(true),
	//}
	//c.Add(s3SaltFile)
	//
	//s3BootstrapScriptFile := &S3File{
	//	Bucket: s3Bucket,
	//	Key:    String("devel/bootstrap"),
	//	Source: findBootstrap(),
	//	Public: Bool(true),
	//}
	//c.Add(s3BootstrapScriptFile)
	//
	//glog.Info("Processing S3 resources")
	//
	//k.ServerBinaryTarURL = s3KubernetesFile.PublicURL()
	//k.ServerBinaryTarHash = s3KubernetesFile.Hash()
	//k.SaltTarURL = s3SaltFile.PublicURL()
	//k.SaltTarHash = s3SaltFile.Hash()
	//k.BootstrapScriptURL = s3BootstrapScriptFile.PublicURL()

	masterPV := &PersistentVolume{
		AvailabilityZone:         String(az),
		Size:       Int64(int64(masterVolumeSize)),
		VolumeType: String(masterVolumeType),
		Name:    String(clusterID + "-master-pd"),
	}

	glog.Info("Processing master volume resource")
	masterPVResources := []fi.Unit{
		masterPV,
	}
	renderItems(context, masterPVResources...)

	k.MasterVolume = target.ReadVar(masterPV)

	iamMasterRole := &IAMRole{
		Name:               String("kubernetes-master"),
		RolePolicyDocument: staticResource("cluster/aws/templates/iam/kubernetes-master-role.json"),
	}
	c.Add(iamMasterRole)

	iamMasterRolePolicy := &IAMRolePolicy{
		Role:           iamMasterRole,
		Name:           String("kubernetes-master"),
		PolicyDocument: staticResource("cluster/aws/templates/iam/kubernetes-master-policy.json"),
	}
	c.Add(iamMasterRolePolicy)

	iamMasterInstanceProfile := &IAMInstanceProfile{
		Name: String("kubernetes-master"),
	}
	c.Add(iamMasterInstanceProfile)

	iamMasterInstanceProfileRole := &IAMInstanceProfileRole{
		InstanceProfile: iamMasterInstanceProfile,
		Role: iamMasterRole,
	}
	c.Add(iamMasterInstanceProfileRole)

	iamNodeRole := &IAMRole{
		Name:             String("kubernetes-minion"),
		RolePolicyDocument: staticResource("cluster/aws/templates/iam/kubernetes-minion-role.json"),
	}
	c.Add(iamNodeRole)

	iamNodeRolePolicy := &IAMRolePolicy{
		Role:           iamNodeRole,
		Name:          String("kubernetes-minion"),
		PolicyDocument: staticResource("cluster/aws/templates/iam/kubernetes-minion-policy.json"),
	}
	c.Add(iamNodeRolePolicy)

	iamNodeInstanceProfile := &IAMInstanceProfile{
		Name:String("kubernetes-minion"),
	}
	c.Add(iamNodeInstanceProfile)

	iamNodeInstanceProfileRole := &IAMInstanceProfileRole{
		InstanceProfile: iamNodeInstanceProfile,
		Role: iamNodeRole,
	}
	c.Add(iamNodeInstanceProfileRole)

	sshKey := &SSHKey{Name: String("kubernetes-" + clusterID), PublicKey: &FileResource{Path: "~/.ssh/justin2015.pub"}}
	c.Add(sshKey)

	vpc := &VPC{CIDR:String("172.20.0.0/16")}
	c.Add(vpc)

	subnet := &Subnet{VPC: vpc, AvailabilityZone: &az, CIDR: String("172.20.0.0/24")}
	c.Add(subnet)

	igw := &InternetGateway{VPC: vpc}
	c.Add(igw)

	routeTable := &RouteTable{VPC: vpc}
	c.Add(routeTable)

	route := &Route{RouteTable: routeTable, CIDR: String("0.0.0.0/0"), InternetGateway: igw}
	c.Add(route)

	masterSG := &SecurityGroup{
		Name:        String("kubernetes-master-" + clusterID),
		Description: String("Security group for master nodes"),
		VPC:         vpc}
	c.Add(masterSG)

	nodeSG := &SecurityGroup{
		Name:        String("kubernetes-minion-" + clusterID),
		Description: String("Security group for minion nodes"),
		VPC:         vpc}
	c.Add(nodeSG)

	masterUserData := &MasterScript{
		Config: k,
	}
	c.Add(masterUserData)

	masterBlockDeviceMappings := []*BlockDeviceMapping{}

	// Be sure to map all the ephemeral drives.  We can specify more than we actually have.
	// TODO: Actually mount the correct number (especially if we have more), though this is non-trivial, and
	//  only affects the big storage instance types, which aren't a typical use case right now.
	for i := 0; i < 4; i++ {
		bdm := &BlockDeviceMapping{
			DeviceName:  String("/dev/sd" + string('c' + i)),
			VirtualName: String("ephemeral" + strconv.Itoa(i)),
		}
		masterBlockDeviceMappings = append(masterBlockDeviceMappings, bdm)
	}

	nodeBlockDeviceMappings := masterBlockDeviceMappings
	nodeUserData := &NodeScript{
		Config: k,
	}
	c.Add(nodeUserData)

	masterInstance := &Instance{
		Name: String(clusterID + "-master"),
		Subnet:              subnet,
		PrivateIPAddress:    String(masterInternalIP),
		InstanceCommonConfig: InstanceCommonConfig{
			SSHKey:              sshKey,
			SecurityGroups:      []*SecurityGroup{masterSG},
			IAMInstanceProfile:  iamMasterInstanceProfile,
			ImageID:             String(imageID),
			InstanceType:        String(masterInstanceType),
			AssociatePublicIP:   Bool(true),
			BlockDeviceMappings: masterBlockDeviceMappings,
			UserData:            masterUserData,
		},
		Tags: map[string]string{"Role": "master"},
	}
	c.Add(masterInstance)

	nodeConfiguration := &AutoscalingLaunchConfiguration{
		Name: String(clusterID + "-minion-group"),
		InstanceCommonConfig: InstanceCommonConfig{
			SSHKey:              sshKey,
			SecurityGroups:      []*SecurityGroup{nodeSG},
			IAMInstanceProfile:  iamNodeInstanceProfile,
			ImageID:             String(imageID),
			InstanceType:        String(nodeInstanceType),
			AssociatePublicIP:   Bool(true),
			BlockDeviceMappings: nodeBlockDeviceMappings,
			UserData:            nodeUserData,
		},
	}
	c.Add(nodeConfiguration)

	nodeGroup := &AutoscalingGroup{
		Name:                String(clusterID + "-minion-group"),
		LaunchConfiguration: nodeConfiguration,
		MinSize:             Int64(int64(nodeCount)),
		MaxSize:             Int64(int64(nodeCount)),
		Subnet:              subnet,
		Tags: map[string]string{
			"Role": "node",
		},
	}
	c.Add(nodeGroup)

	c.Add(masterSG.AllowFrom(masterSG))
	c.Add(masterSG.AllowFrom(nodeSG))
	c.Add(nodeSG.AllowFrom(masterSG))
	c.Add(nodeSG.AllowFrom(nodeSG))

	// SSH is open to the world
	c.Add(nodeSG.AllowTCP("0.0.0.0/0", 22, 22))
	c.Add(masterSG.AllowTCP("0.0.0.0/0", 22, 22))

	// HTTPS to the master is allowed (for API access)
	c.Add(masterSG.AllowTCP("0.0.0.0/0", 443, 443))
}

func staticResource(key string) Resource {
	p := path.Join(basePath, key)
	return &FileResource{Path: p}
}

func findKubernetesTarGz() Resource {
	// TODO: Bash script has a fallback procedure
	path := "_output/release-tars/kubernetes-server-linux-amd64.tar.gz"
	return &FileResource{Path: path}
}

func findSaltTarGz() Resource {
	// TODO: Bash script has a fallback procedure
	path := "_output/release-tars/kubernetes-salt.tar.gz"
	return &FileResource{Path: path}
}

func findBootstrap() Resource {
	path := "bin/bootstrap"
	return &FileResource{Path: path}
}

