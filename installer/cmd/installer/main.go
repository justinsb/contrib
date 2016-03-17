package main

import (
	"flag"
	"log"
	"net"
	"os"
	"path"

	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/config"
	"k8s.io/contrib/installer/pkg/tasks"
)

var basePath string

func staticResource(key string) tasks.Resource {
	p := path.Join(basePath, key)
	return &tasks.FileResource{Path: p}
}

func findKubernetesTarGz() tasks.Resource {
	// TODO: Bash script has a fallback procedure
	path := "_output/release-tars/kubernetes-server-linux-amd64.tar.gz"
	return &tasks.FileResource{Path: path}
}

func findSaltTarGz() tasks.Resource {
	// TODO: Bash script has a fallback procedure
	path := "_output/release-tars/kubernetes-salt.tar.gz"
	return &tasks.FileResource{Path: path}
}

func findBootstrap() tasks.Resource {
	path := "bin/bootstrap"
	return &tasks.FileResource{Path: path}
}

func renderItems(items []tasks.Item, context *tasks.Context) {
	for _, resource := range items {
		glog.Info("rendering ", resource)
		err := resource.Run(context)
		if err != nil {
			glog.Fatalf("error rendering resource (%v): %v", resource, err)
		}
	}
}

func String(s string) *string {
	return &s
}

func main() {
	var config config.Configuration
	var masterVolumeSize int
	var volumeType string
	var s3BucketName string
	var s3Region string

	var minionCount int

	basePath = "/Users/justinsb/k8s/src/github.com/GoogleCloudPlatform/kubernetes/"

	flag.StringVar(&config.Zone, "az", "us-east-1b", "AWS availability zone")
	flag.StringVar(&s3BucketName, "s3-bucket", "", "S3 bucket for upload of artifacts")
	flag.StringVar(&s3Region, "s3-region", "", "Region in which to create the S3 bucket (if it does not exist)")
	flag.BoolVar(&config.EnableClusterUI, "enable-cluster-ui", true, "Enable cluster UI")
	flag.BoolVar(&config.EnableClusterDNS, "enable-cluster-dns", true, "Enable cluster DNS")
	flag.BoolVar(&config.EnableClusterLogging, "enable-cluster-logging", true, "Enable cluster logging")
	flag.StringVar(&config.LoggingDestination, "logging-destination", "elasticsearch", "Default logging destination")
	flag.StringVar(&config.EnableClusterMonitoring, "enable-cluster-monitoring", "influxdb", "Set to enable monitoring")
	flag.BoolVar(&config.EnableNodeLogging, "enable-node-logging", true, "Enable node logging")
	flag.IntVar(&config.ElasticsearchLoggingReplicas, "elasticsearch-logging-replicas", 1, "Replicas to create for elasticsearch cluster")

	flag.IntVar(&config.DNSReplicas, "dns-replicas", 1, "Number of replicas for DNS")
	flag.StringVar(&config.DNSServerIP, "dns-server-ip", "10.0.0.10", "Service IP for DNS")
	//flag.StringVar(&config.DNSDomain, "dns-domain", "cluster.local", "Domain for internal service DNS")

	flag.StringVar(&config.AdmissionControl, "admission-control", "NamespaceLifecycle,NamespaceExists,LimitRanger,SecurityContextDeny,ServiceAccount,ResourceQuota", "Admission control policies")

	flag.StringVar(&config.ServiceClusterIPRange, "service-cluster-ip-range", "10.0.0.0/16", "IP range to assign to services")
	flag.StringVar(&config.ClusterIPRange, "cluster-ip-range", "10.244.0.0/16", "IP range for in-cluster (pod) IPs")
	flag.StringVar(&config.MasterCIDR, "master-ip-range", "10.246.0.0/24", "IP range for master in-cluster (pod) IPs")

	flag.StringVar(&config.DockerStorage, "docker-storage", "aufs", "Filesystem to use for docker storage")

	flag.StringVar(&config.ClusterID, "cluster-id", "", "cluster id")
	flag.StringVar(&volumeType, "volume-type", "gp2", "Type for EBS volumes")
	flag.IntVar(&masterVolumeSize, "master-volume-size", 20, "Size for master volume")
	flag.IntVar(&minionCount, "minion-count", 2, "Number of minions")

	flag.Set("alsologtostderr", "true")

	flag.Parse()

	clusterID := config.ClusterID
	if clusterID == "" {
		glog.Exit("cluster-id is required")
	}

	az := config.Zone
	if len(az) <= 2 {
		glog.Exit("Invalid AZ: ", az)
	}
	region := az[:len(az)-1]

	if s3BucketName == "" {
		// TODO: Implement the generation logic
		glog.Exit("s3-bucket is required (for now!)")
	}

	if s3Region == "" {
		s3Region = region
	}

	// Required to work with autoscaling minions
	config.AllocateNodeCIDRs = true

	// Simplifications
	config.DNSDomain = "cluster.local"
	instancePrefix := config.ClusterID
	config.InstancePrefix = instancePrefix
	masterInternalIP := "172.20.0.9"
	config.SaltMaster = masterInternalIP
	config.MasterInternalIP = masterInternalIP
	nodeInstancePrefix := instancePrefix + "-minion"
	config.NodeInstancePrefix = nodeInstancePrefix
	config.MasterName = instancePrefix + "-master"

	config.CloudProvider = "aws"

	config.KubeUser = "admin"
	config.KubePassword = tasks.RandomToken(16)

	config.KubeletToken = tasks.RandomToken(32)
	config.KubeProxyToken = tasks.RandomToken(32)

	serviceIP, _, err := net.ParseCIDR(config.ServiceClusterIPRange)
	if err != nil {
		glog.Fatalf("Error parsing service-cluster-ip-range: %v", err)
	}
	serviceIP[len(serviceIP)-1]++

	masterExtraSans := []string{
		"IP:" + serviceIP.String(),
		"DNS:kubernetes",
		"DNS:kubernetes.default",
		"DNS:kubernetes.default.svc",
		"DNS:kubernetes.default.svc." + config.DNSDomain,
		"DNS:" + config.MasterName,
	}
	config.MasterExtraSans = masterExtraSans

	distro := &tasks.DistroVivid{}
	imageID := distro.GetImageID(region)

	if imageID == "" {
		log.Fatal("ImageID could not be determined")
	}

	tags := map[string]string{"KubernetesCluster": clusterID}
	cloud := tasks.NewAWSCloud(region, tags)

	target := tasks.NewBashTarget(cloud)

	context := tasks.NewContext(target, cloud)

	vpc := &tasks.VPC{CIDR: String("172.20.0.0/16")}
	subnet := &tasks.Subnet{VPC: vpc, AvailabilityZone: String(az), CIDR: String("172.20.0.0/24")}
	igw := &tasks.InternetGateway{VPC: vpc}
	routeTable := &tasks.RouteTable{VPC: vpc}
	rta := &tasks.RouteTableAssociation{RouteTable: routeTable, Subnet: subnet}
	route := &tasks.Route{RouteTable: routeTable, CIDR: String("0.0.0.0/0"), InternetGateway: igw}
	masterSG := &tasks.SecurityGroup{
		Name:        String("kubernetes-master-" + clusterID),
		Description: String("Security group for master nodes"),
		VPC:         vpc}
	minionSG := &tasks.SecurityGroup{
		Name:        String("kubernetes-minion-" + clusterID),
		Description: String("Security group for minion nodes"),
		VPC:         vpc}

	glog.Info("Processing VPC resources")
	resources := []tasks.Item{vpc, subnet, igw, routeTable, rta, route, masterSG, minionSG}

	resources = append(resources, masterSG.AllowFrom(masterSG))
	resources = append(resources, masterSG.AllowFrom(minionSG))
	resources = append(resources, minionSG.AllowFrom(masterSG))
	resources = append(resources, minionSG.AllowFrom(minionSG))

	iamMasterRole := &tasks.IAMRole{
		Name:               "kubernetes-master",
		RolePolicyDocument: staticResource("cluster/aws/templates/iam/kubernetes-master-role.json"),
	}
	resources = append(resources, iamMasterRole)

	renderItems(resources, context)

	/*
		instanceType := "m3.medium"

		s3Bucket := &tasks.S3Bucket{
			Name:         s3BucketName,
			CreateRegion: s3Region,
		}

		s3KubernetesFile := &tasks.S3File{
			Bucket: s3Bucket,
			Key:    "devel/kubernetes-server-linux-amd64.tar.gz",
			Source: findKubernetesTarGz(),
			Public: true,
		}

		s3SaltFile := &tasks.S3File{
			Bucket: s3Bucket,
			Key:    "devel/kubernetes-salt.tar.gz",
			Source: findSaltTarGz(),
			Public: true,
		}

		s3BootstrapFile := &tasks.S3File{
			Bucket: s3Bucket,
			Key:    "devel/kubernetes-bootstrap",
			Source: findBootstrap(),
			Public: true,
		}

		s3Resources := []tasks.Item{
			s3Bucket,
			s3KubernetesFile,
			s3BootstrapFile,
			s3SaltFile,
		}

		glog.Info("Processing S3 resources")
		renderItems(s3Resources, cloud, target)

		config.ServerBinaryTarURL = s3KubernetesFile.PublicURL()
		config.SaltTarURL = s3SaltFile.PublicURL()
		config.BootstrapURL = s3BootstrapFile.PublicURL()

		masterPV := &tasks.PersistentVolume{
			AZ:         az,
			Size:       masterVolumeSize,
			VolumeType: volumeType,
			NameTag:    clusterID + "-master-pd",
		}

		glog.Info("Processing master volume resource")
		masterPVResources := []tasks.BashRenderable{
			masterPV,
		}
		renderItems(masterPVResources, cloud, target)

		config.MasterVolume = target.ReadVar(masterPV)

		iamMasterRole := &tasks.IAMRole{
			Name:               "kubernetes-master",
			RolePolicyDocument: staticResource("cluster/aws/templates/iam/kubernetes-master-role.json"),
		}
		iamMasterRolePolicy := &tasks.IAMRolePolicy{
			Role:           iamMasterRole,
			Name:           "kubernetes-master",
			PolicyDocument: staticResource("cluster/aws/templates/iam/kubernetes-master-policy.json"),
		}
		iamMasterInstanceProfile := &tasks.IAMInstanceProfile{
			Name: "kubernetes-master",
			Role: iamMasterRole,
		}

		iamMinionRole := &tasks.IAMRole{
			Name:               "kubernetes-minion",
			RolePolicyDocument: staticResource("cluster/aws/templates/iam/kubernetes-minion-role.json"),
		}
		iamMinionRolePolicy := &tasks.IAMRolePolicy{
			Role:           iamMinionRole,
			Name:           "kubernetes-minion",
			PolicyDocument: staticResource("cluster/aws/templates/iam/kubernetes-minion-policy.json"),
		}
		iamMinionInstanceProfile := &tasks.IAMInstanceProfile{
			Name: "kubernetes-minion",
			Role: iamMinionRole,
		}

		sshKey := &tasks.SSHKey{Name: "kubernetes-" + clusterID, PublicKey: &tasks.FileResource{Path: "~/.ssh/justin2015.pub"}}
		vpc := &tasks.VPC{CIDR: "172.20.0.0/16"}
		subnet := &tasks.Subnet{VPC: vpc, AZ: az, CIDR: "172.20.0.0/24"}
		igw := &tasks.InternetGateway{VPC: vpc}
		routeTable := &tasks.RouteTable{Subnet: subnet}
		route := &tasks.Route{RouteTable: routeTable, CIDR: "0.0.0.0/0", InternetGateway: igw}
		masterSG := &tasks.SecurityGroup{
			Name:        "kubernetes-master-" + clusterID,
			Description: "Security group for master nodes",
			VPC:         vpc}
		minionSG := &tasks.SecurityGroup{
			Name:        "kubernetes-minion-" + clusterID,
			Description: "Security group for minion nodes",
			VPC:         vpc}

		masterUserData := &tasks.MasterScript{
			Config: &config,
		}

		masterBlockDeviceMappings := []ec2.BlockDeviceMapping{}

		// Be sure to map all the ephemeral drives.  We can specify more than we actually have.
		// TODO: Actually mount the correct number (especially if we have more), though this is non-trivial, and
		//  only affects the big storage instance types, which aren't a typical use case right now.
		for i := 0; i < 4; i++ {
			bdm := &ec2.BlockDeviceMapping{
				DeviceName:  aws.String("/dev/sd" + string('c'+i)),
				VirtualName: aws.String("ephemeral" + strconv.Itoa(i)),
			}
			masterBlockDeviceMappings = append(masterBlockDeviceMappings, *bdm)
		}

		minionBlockDeviceMappings := masterBlockDeviceMappings
		minionUserData := &tasks.MinionScript{
			Config: &config,
		}

		masterInstance := &tasks.Instance{
			NameTag: clusterID + "-master",
			InstanceConfig: tasks.InstanceConfig{
				Subnet:              subnet,
				SSHKey:              sshKey,
				SecurityGroups:      []*tasks.SecurityGroup{masterSG},
				IAMInstanceProfile:  iamMasterInstanceProfile,
				ImageID:             imageID,
				InstanceType:        instanceType,
				AssociatePublicIP:   true,
				PrivateIPAddress:    masterInternalIP,
				BlockDeviceMappings: masterBlockDeviceMappings,
				UserData:            masterUserData,
			},
			Tags: map[string]string{"Role": "master"},
		}

		minionConfiguration := &tasks.AutoscalingLaunchConfiguration{
			Name: clusterID + "-minion-group",
			InstanceConfig: tasks.InstanceConfig{
				SSHKey:              sshKey,
				SecurityGroups:      []*tasks.SecurityGroup{minionSG},
				IAMInstanceProfile:  iamMinionInstanceProfile,
				ImageID:             imageID,
				InstanceType:        instanceType,
				AssociatePublicIP:   true,
				BlockDeviceMappings: minionBlockDeviceMappings,
				UserData:            minionUserData,
			},
		}

		minionGroup := &tasks.AutoscalingGroup{
			Name:                clusterID + "-minion-group",
			LaunchConfiguration: minionConfiguration,
			MinSize:             minionCount,
			MaxSize:             minionCount,
			Subnet:              subnet,
			Tags: map[string]string{
				"Role": "minion",
			},
		}

		resources := []tasks.BashRenderable{
			iamMasterRole, iamMasterRolePolicy, iamMasterInstanceProfile,
			iamMinionRole, iamMinionRolePolicy, iamMinionInstanceProfile,
			sshKey, vpc, subnet, igw,
			routeTable, route,
			masterSG, minionSG,
			masterInstance,

			minionConfiguration,
			minionGroup,
		}

		resources = append(resources, masterSG.AllowFrom(masterSG))
		resources = append(resources, masterSG.AllowFrom(minionSG))
		resources = append(resources, minionSG.AllowFrom(masterSG))
		resources = append(resources, minionSG.AllowFrom(minionSG))

		// SSH is open to the world
		resources = append(resources, minionSG.AllowTCP("0.0.0.0/0", 22, 22))
		resources = append(resources, masterSG.AllowTCP("0.0.0.0/0", 22, 22))

		// HTTPS to the master is allowed (for API access)
		resources = append(resources, masterSG.AllowTCP("0.0.0.0/0", 443, 443))

		glog.Info("Processing main resources")
		renderItems(resources, cloud, target)
	*/

	target.DebugDump()

	err = target.PrintShellCommands(os.Stdout)
	if err != nil {
		glog.Fatal("error building shell commands: %v", err)
	}
}
