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
	"strconv"
	"k8s.io/contrib/installer/pkg/fi"
)

var basePath string


func main() {
	var config config.Configuration


	basePath = "/Users/justinsb/k8s/src/github.com/GoogleCloudPlatform/kubernetes/"

	var configPath string
	flag.StringVar(&configPath, "config", configPath, "Path to config file")

	flag.StringVar(&config.Zone, "az", "us-east-1b", "AWS availability zone")
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

	flag.Set("alsologtostderr", "true")

	flag.Parse()





	// Required to work with autoscaling minions
	config.AllocateNodeCIDRs = true

	// Simplifications
	config.DNSDomain = "cluster.local"
	instancePrefix := config.ClusterID
	config.InstancePrefix = instancePrefix

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
	serviceIP[len(serviceIP) - 1]++

	masterExtraSans := []string{
		"IP:" + serviceIP.String(),
		"DNS:kubernetes",
		"DNS:kubernetes.default",
		"DNS:kubernetes.default.svc",
		"DNS:kubernetes.default.svc." + config.DNSDomain,
		"DNS:" + config.MasterName,
	}
	config.MasterExtraSans = masterExtraSans



	tags := map[string]string{"KubernetesCluster": clusterID}
	cloud := fi.NewAWSCloud(region, tags)

	target := tasks.NewBashTarget(cloud)

	// TODO: Rationalize configs
	fiConfig := fi.NewSimpleConfig()
	if configPath != "" {
		err := fiConfig.ReadYaml(configPath)
		if err != nil {
			glog.Fatalf("error reading configuration: %v", err)
		}
	}

	fiContext, err := fi.NewContext(fiConfig, cloud)
	if err != nil {
		glog.Fatalf("error building config: %v", err)
	}

	context := fiContext.NewRunContext(target, fi.ModeConfigure)

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

	glog.Info("Processing main resources")
	renderItems(context, resources...)

	target.DebugDump()

	err = target.PrintShellCommands(os.Stdout)
	if err != nil {
		glog.Fatal("error building shell commands: %v", err)
	}
}
