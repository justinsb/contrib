package main

import (
	"flag"
	"net"
	"os"

	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/tasks"
	"k8s.io/contrib/installer/pkg/fi"
	"k8s.io/contrib/installer/pkg/fi/filestore"
	"k8s.io/contrib/installer/pkg/fi/ca"
)

//var templateDir = "templates"

func main() {
	k := &tasks.K8s{}
	k.Init()

	var configPath string
	flag.StringVar(&configPath, "config", configPath, "Path to config file")

	var s3Region string
	flag.StringVar(&s3Region, "s3-region", "", "Region in which to create the S3 bucket (if it does not exist)")

	var s3BucketName string
	flag.StringVar(&s3BucketName, "s3-bucket", "", "S3 bucket for upload of artifacts")

	flag.StringVar(&k.ClusterID, "cluster-id", "", "cluster id")

	flag.StringVar(&k.Zone, "az", k.Zone, "AWS availability zone")
	flag.BoolVar(&k.EnableClusterUI, "enable-cluster-ui", k.EnableClusterUI, "Enable cluster UI")
	flag.BoolVar(&k.EnableClusterDNS, "enable-cluster-dns", k.EnableClusterDNS, "Enable cluster DNS")
	flag.BoolVar(&k.EnableClusterLogging, "enable-cluster-logging", k.EnableClusterLogging, "Enable cluster logging")
	flag.StringVar(&k.LoggingDestination, "logging-destination", k.LoggingDestination, "Default logging destination")
	flag.StringVar(&k.EnableClusterMonitoring, "enable-cluster-monitoring", k.EnableClusterMonitoring, "Set to enable monitoring")
	flag.BoolVar(&k.EnableNodeLogging, "enable-node-logging", k.EnableNodeLogging, "Enable node logging")
	flag.IntVar(&k.ElasticsearchLoggingReplicas, "elasticsearch-logging-replicas", k.ElasticsearchLoggingReplicas, "Replicas to create for elasticsearch cluster")

	flag.IntVar(&k.DNSReplicas, "dns-replicas", k.DNSReplicas, "Number of replicas for DNS")
	flag.StringVar(&k.DNSServerIP, "dns-server-ip", k.DNSServerIP, "Service IP for DNS")
	flag.StringVar(&k.DNSDomain, "dns-domain", k.DNSDomain, "Domain for internal service DNS")

	flag.StringVar(&k.AdmissionControl, "admission-control", k.AdmissionControl, "Admission control policies")

	flag.StringVar(&k.ServiceClusterIPRange, "service-cluster-ip-range", k.ServiceClusterIPRange, "IP range to assign to services")
	flag.StringVar(&k.ClusterIPRange, "cluster-ip-range", k.ClusterIPRange, "IP range for in-cluster (pod) IPs")
	flag.StringVar(&k.MasterCIDR, "master-ip-range", k.MasterIPRange, "IP range for master in-cluster (pod) IPs")

	flag.StringVar(&k.DockerStorage, "docker-storage", k.DockerStorage, "Filesystem to use for docker storage")

	flag.Set("alsologtostderr", "true")

	flag.Parse()



	// Required to work with autoscaling minions
	k.AllocateNodeCIDRs = true

	// Simplifications
	instancePrefix := k.ClusterID
	k.InstancePrefix = instancePrefix

	nodeInstancePrefix := instancePrefix + "-minion"
	k.NodeInstancePrefix = nodeInstancePrefix
	k.MasterName = instancePrefix + "-master"

	k.CloudProvider = "aws"

	k.KubeUser = "admin"
	k.KubePassword = tasks.RandomToken(16)

	k.KubeletToken = tasks.RandomToken(32)
	k.KubeProxyToken = tasks.RandomToken(32)

	serviceIP, _, err := net.ParseCIDR(k.ServiceClusterIPRange)
	if err != nil {
		glog.Fatalf("Error parsing service-cluster-ip-range: %v", err)
	}
	serviceIP[len(serviceIP) - 1]++

	masterExtraSans := []string{
		"IP:" + serviceIP.String(),
		"DNS:kubernetes",
		"DNS:kubernetes.default",
		"DNS:kubernetes.default.svc",
		"DNS:kubernetes.default.svc." + k.DNSDomain,
		"DNS:" + k.MasterName,
	}
	k.MasterExtraSans = masterExtraSans

	az := k.Zone
	if len(az) <= 2 {
		glog.Exit("Invalid AZ: ", az)
	}
	region := az[:len(az) - 1]
	if s3Region == "" {
		s3Region = region
	}

	if s3BucketName == "" {
		// TODO: Implement the generation logic
		glog.Exit("s3-bucket is required (for now!)")
	}

	tags := map[string]string{"KubernetesCluster": k.ClusterID}
	cloud := fi.NewAWSCloud(region, tags)

	s3Bucket, err := cloud.S3.EnsureBucket(s3BucketName, s3Region)
	if err != nil {
		glog.Exitf("error creating s3 bucket: %v", err)
	}
	s3Prefix := "devel/" + k.ClusterID + "/"
	filestore := filestore.NewS3FileStore(s3Bucket, s3Prefix)
	castore, err := ca.NewCAStore("pki")
	if err != nil {
		glog.Exitf("error building CA store: %v", err)
	}
	target := tasks.NewBashTarget(cloud, filestore)

	// TODO: Rationalize configs
	config := fi.NewSimpleConfig()
	if configPath != "" {
		panic("additional config not supported yet")
		//err := fi.Config.ReadYaml(configPath)
		//if err != nil {
		//	glog.Fatalf("error reading configuration: %v", err)
		//}
	}

	context, err := fi.NewContext(config, cloud, castore)
	if err != nil {
		glog.Fatalf("error building config: %v", err)
	}

	/*
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
*/

	bc := context.NewBuildContext()
	bc.Add(k)

	runMode := fi.ModeConfigure
	//if validate {
	//	runMode = fi.ModeValidate
	//}

	rc := context.NewRunContext(target, runMode)
	err = rc.Run()
	if err != nil {
		glog.Fatalf("error running configuration: %v", err)
	}

	target.DebugDump()

	err = target.PrintShellCommands(os.Stdout)
	if err != nil {
		glog.Fatal("error building shell commands: %v", err)
	}
}
