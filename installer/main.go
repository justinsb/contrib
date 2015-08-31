package main

import (
	"bytes"
	crypto_rand "crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/golang/glog"
)

func randomToken(length int) string {
	// This is supposed to be the same algorithm as the old bash algorithm
	// KUBELET_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)
	// KUBE_PROXY_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)

	for {
		buffer := make([]byte, length*4)
		_, err := crypto_rand.Read(buffer)
		if err != nil {
			glog.Fatalf("error generating random token: %v", err)
		}
		s := base64.StdEncoding.EncodeToString(buffer)
		var trimmed bytes.Buffer
		for _, c := range s {
			switch c {
			case '=', '+', '/':
				continue
			default:
				trimmed.WriteRune(c)
			}
		}

		s = string(trimmed.Bytes())
		if len(s) >= length {
			return s[0:length]
		}
	}
}

var templateDir = "templates"

type BashRenderable interface {
	RenderBash(cloud *AWSCloud, output *BashTarget) error
}

type SSHKey struct {
	Name      string
	PublicKey Resource
}

func (k *SSHKey) String() string {
	return fmt.Sprintf("SSHKey (name=%s)", k.Name)
}

type VPC struct {
	CIDR string
}

func (v *VPC) Prefix() string {
	return "Vpc"
}

func (v *VPC) String() string {
	return fmt.Sprintf("VPC (cidr=%s)", v.CIDR)
}

func (v *VPC) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	request := &ec2.DescribeVpcsInput{
		Filters: cloud.BuildFilters(),
	}

	response, err := cloud.ec2.DescribeVpcs(request)
	if err != nil {
		return fmt.Errorf("error listing VPCs: %v", err)
	}

	var existing *ec2.Vpc
	if response != nil && len(response.Vpcs) != 0 {
		if len(response.Vpcs) != 1 {
			glog.Fatalf("found multiple VPCs matching tags")
		}
		glog.V(2).Info("found matching VPC")
		existing = response.Vpcs[0]
	}

	vpcId := ""
	output.CreateVar(v)
	if existing == nil {
		glog.V(2).Info("VPC not found; will create")
		output.AddEC2Command("create-vpc", "--cidr-block", v.CIDR, "--query", "Vpc.VpcId").AssignTo(v)
	} else {
		vpcId = aws.StringValue(existing.VpcId)
		output.AddAssignment(v, vpcId)
	}

	hasDnsSupport := false
	if existing != nil {
		request := &ec2.DescribeVpcAttributeInput{VpcId: existing.VpcId, Attribute: aws.String(ec2.VpcAttributeNameEnableDnsSupport)}
		response, err := cloud.ec2.DescribeVpcAttribute(request)
		if err != nil {
			return fmt.Errorf("error querying for dns support: %v", err)
		}
		if response != nil && response.EnableDnsSupport != nil {
			hasDnsSupport = aws.BoolValue(response.EnableDnsSupport.Value)
		}
	}

	if !hasDnsSupport {
		output.AddEC2Command("modify-vpc-attribute", "--vpc-id", output.ReadVar(v), "--enable-dns-support", "'{\"Value\": true}'")
	}

	hasDnsHostnames := false
	if existing != nil {
		request := &ec2.DescribeVpcAttributeInput{VpcId: existing.VpcId, Attribute: aws.String(ec2.VpcAttributeNameEnableDnsHostnames)}
		response, err := cloud.ec2.DescribeVpcAttribute(request)
		if err != nil {
			return fmt.Errorf("error querying for dns hostnames: %v", err)
		}
		if response != nil && response.EnableDnsHostnames != nil {
			hasDnsHostnames = aws.BoolValue(response.EnableDnsHostnames.Value)
		}
	}

	if !hasDnsHostnames {
		output.AddEC2Command("modify-vpc-attribute", "--vpc-id", output.ReadVar(v), "--enable-dns-hostnames", "'{\"Value\": true}'")
	}

	return output.AddAWSTags(cloud.Tags(), v, "vpc")
}

type Subnet struct {
	VPC  *VPC
	CIDR string
	AZ   string
}

func (s *Subnet) Prefix() string {
	return "Subnet"
}

func (s *Subnet) String() string {
	return fmt.Sprintf("Subnet (cidr=%s)", s.CIDR)
}

func (s *Subnet) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	request := &ec2.DescribeSubnetsInput{
		Filters: cloud.BuildFilters(),
	}

	response, err := cloud.ec2.DescribeSubnets(request)
	if err != nil {
		return fmt.Errorf("error listing subnets: %v", err)
	}

	var existing *ec2.Subnet
	if response != nil && len(response.Subnets) != 0 {
		if len(response.Subnets) != 1 {
			glog.Fatalf("found multiple subnets matching tags")
		}
		glog.V(2).Info("found matching subnet")
		existing = response.Subnets[0]
	}

	subnetId := ""
	output.CreateVar(s)
	if existing == nil {
		glog.V(2).Info("Subnet not found; will create")
		output.AddEC2Command("create-subnet", "--cidr-block", s.CIDR, "--availability-zone", s.AZ, "--vpc-id", output.ReadVar(s.VPC), "--query", "Subnet.SubnetId").AssignTo(s)
	} else {
		subnetId = aws.StringValue(existing.SubnetId)
		output.AddAssignment(s, subnetId)
	}

	return output.AddAWSTags(cloud.Tags(), s, "subnet")
}

type AWSCloud struct {
	region      string
	s3          *s3.S3
	iam         *iam.IAM
	ec2         *ec2.EC2
	autoscaling *autoscaling.AutoScaling
	tags        map[string]string
}

func NewAWSCloud(region string, tags map[string]string) *AWSCloud {
	c := &AWSCloud{region: region}
	config := aws.NewConfig().WithRegion(region)
	c.ec2 = ec2.New(config)
	c.s3 = s3.New(config)
	c.iam = iam.New(config)
	c.autoscaling = autoscaling.New(config)
	c.tags = tags
	return c
}

func newEc2Filter(name string, values ...string) *ec2.Filter {
	awsValues := []*string{}
	for _, value := range values {
		awsValues = append(awsValues, aws.String(value))
	}
	filter := &ec2.Filter{
		Name:   aws.String(name),
		Values: awsValues,
	}
	return filter
}

func (c *AWSCloud) Tags() map[string]string {
	// Defensive copy
	tags := make(map[string]string)
	for k, v := range c.tags {
		tags[k] = v
	}
	return tags
}

func (c *AWSCloud) GetTags(resourceId string, resourceType string) (map[string]string, error) {
	tags := map[string]string{}

	request := &ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			newEc2Filter("resource-id", resourceId),
			newEc2Filter("resource-type", resourceType),
		},
	}

	response, err := c.ec2.DescribeTags(request)
	if err != nil {
		return nil, fmt.Errorf("error listing tags on %v:%v: %v", resourceType, resourceId, err)
	}

	for _, tag := range response.Tags {
		if tag == nil {
			glog.Warning("unexpected nil tag")
			continue
		}
		tags[aws.StringValue(tag.Key)] = aws.StringValue(tag.Value)
	}

	return tags, nil
}

func (c *AWSCloud) BuildFilters() []*ec2.Filter {
	filters := []*ec2.Filter{}
	for name, value := range c.tags {
		filter := newEc2Filter("tag:"+name, value)
		filters = append(filters, filter)
	}
	return filters
}

func (c *AWSCloud) EnvVars() map[string]string {
	env := map[string]string{}
	env["AWS_DEFAULT_REGION"] = c.region
	env["AWS_DEFAULT_OUTPUT"] = "text"
	return env
}

type BashTarget struct {
	cloud    *AWSCloud
	commands []*BashCommand

	ec2Args              []string
	autoscalingArgs      []string
	iamArgs              []string
	vars                 map[HasId]*BashVar
	prefixCounts         map[string]int
	resourcePrefixCounts map[string]int
}

func NewBashTarget(cloud *AWSCloud) *BashTarget {
	b := &BashTarget{cloud: cloud}
	b.ec2Args = []string{"aws", "ec2"}
	b.autoscalingArgs = []string{"aws", "autoscaling"}
	b.iamArgs = []string{"aws", "iam"}
	b.vars = make(map[HasId]*BashVar)
	b.prefixCounts = make(map[string]int)
	b.resourcePrefixCounts = make(map[string]int)
	return b
}

type HasId interface {
	Prefix() string
}

type BashVar struct {
	name        string
	staticValue *string
}

func (t *BashTarget) CreateVar(v HasId) *BashVar {
	bv, found := t.vars[v]
	if found {
		glog.Fatal("Attempt to create variable twice for ", v)
	}
	bv = &BashVar{}
	prefix := strings.ToUpper(v.Prefix())
	n := t.prefixCounts[prefix]
	n++
	t.prefixCounts[prefix] = n

	bv.name = prefix + "_" + strconv.Itoa(n)
	t.vars[v] = bv
	return bv
}

type BashCommand struct {
	parent   *BashTarget
	args     []string
	assignTo *BashVar
}

func (c *BashCommand) AssignTo(s HasId) *BashCommand {
	bv := c.parent.vars[s]
	if bv == nil {
		glog.Fatal("no variable assigned to ", s)
	}
	c.assignTo = bv
	return c
}

func (c *BashCommand) DebugDump() {
	if c.assignTo != nil {
		glog.Info("CMD: ", c.assignTo.name, "=`", c.args, "`")
	} else {
		glog.Info("CMD: ", c.args)
	}
}

func (c *BashCommand) PrintShellCommand(w io.Writer) error {
	var buf bytes.Buffer

	if c.assignTo != nil {
		buf.WriteString(c.assignTo.name)
		buf.WriteString("=`")
	}

	for i, arg := range c.args {
		if i != 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(arg)
	}

	if c.assignTo != nil {
		buf.WriteString("`")
	}

	buf.WriteString("\n")

	_, err := buf.WriteTo(w)
	return err
}

func (t *BashTarget) ReadVar(s HasId) string {
	bv := t.vars[s]
	if bv == nil {
		glog.Fatal("no variable assigned to ", s)
	}

	// TODO: Escaping?
	return "${" + bv.name + "}"
}

func (t *BashTarget) DebugDump() {
	for _, cmd := range t.commands {
		cmd.DebugDump()
	}
}

func (t *BashTarget) PrintShellCommands(w io.Writer) error {
	var header bytes.Buffer
	header.WriteString("#!/bin/bash\n")
	header.WriteString("set -ex\n\n")
	header.WriteString(". ./helpers\n\n")

	for k, v := range t.cloud.EnvVars() {
		header.WriteString("export " + k + "=" + bashQuoteString(v) + "\n")
	}

	_, err := header.WriteTo(w)
	if err != nil {
		return err
	}

	for _, cmd := range t.commands {
		err = cmd.PrintShellCommand(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *BashTarget) AddEC2Command(args ...string) *BashCommand {
	cmd := &BashCommand{parent: t}
	cmd.args = t.ec2Args
	cmd.args = append(cmd.args, args...)

	return t.AddCommand(cmd)
}

func (t *BashTarget) AddAutoscalingCommand(args ...string) *BashCommand {
	cmd := &BashCommand{parent: t}
	cmd.args = t.autoscalingArgs
	cmd.args = append(cmd.args, args...)

	return t.AddCommand(cmd)
}

func (t *BashTarget) AddS3Command(region string, args ...string) *BashCommand {
	cmd := &BashCommand{parent: t}
	cmd.args = []string{"aws", "s3", "--region", region}
	cmd.args = append(cmd.args, args...)

	return t.AddCommand(cmd)
}

func (t *BashTarget) AddS3APICommand(region string, args ...string) *BashCommand {
	cmd := &BashCommand{parent: t}
	cmd.args = []string{"aws", "s3api", "--region", region}
	cmd.args = append(cmd.args, args...)

	return t.AddCommand(cmd)
}

func (t *BashTarget) AddIAMCommand(args ...string) *BashCommand {
	cmd := &BashCommand{parent: t}
	cmd.args = t.iamArgs
	cmd.args = append(cmd.args, args...)

	return t.AddCommand(cmd)
}

func bashQuoteString(s string) string {
	// TODO: Escaping
	var quoted bytes.Buffer
	for _, c := range s {
		switch c {
		case '"':
			quoted.WriteString("\\\"")
		default:
			quoted.WriteString(string(c))
		}
	}

	return "\"" + string(quoted.Bytes()) + "\""
}

func (t *BashTarget) AddAWSTags(expected map[string]string, s HasId, resourceType string) error {
	resourceId, exists := t.FindValue(s)
	var missing map[string]string
	if exists {
		actual, err := t.cloud.GetTags(resourceId, resourceType)
		if err != nil {
			return fmt.Errorf("unexpected error fetchin tags for resource: %v", err)
		}

		missing := map[string]string{}
		for k, v := range expected {
			actualValue, found := actual[k]
			if found && actualValue == v {
				continue
			}
			missing[k] = v
		}
	} else {
		missing = expected
	}

	for name, value := range missing {
		cmd := &BashCommand{}
		cmd.args = []string{"add-tag", t.ReadVar(s), bashQuoteString(name), bashQuoteString(value)}
		t.AddCommand(cmd)
	}

	return nil
}

func (t *BashTarget) AddCommand(cmd *BashCommand) *BashCommand {
	t.commands = append(t.commands, cmd)

	return cmd
}

func (t *BashTarget) AddAssignment(h HasId, value string) {
	bv := t.vars[h]
	if bv == nil {
		glog.Fatal("no variable assigned to ", h)
	}

	cmd := &BashCommand{}
	assign := bv.name + "=" + bashQuoteString(value)
	cmd.args = []string{assign}
	t.AddCommand(cmd)

	bv.staticValue = &value
}

func (t *BashTarget) FindValue(h HasId) (string, bool) {
	bv := t.vars[h]
	if bv == nil {
		glog.Fatal("no variable assigned to ", h)
	}

	if bv.staticValue == nil {
		return "", false
	}
	return *bv.staticValue, true
}

func (t *BashTarget) generateDynamicPath(prefix string) string {
	basePath := "resources"
	n := t.resourcePrefixCounts[prefix]
	n++
	t.resourcePrefixCounts[prefix] = n

	name := prefix + "_" + strconv.Itoa(n)
	p := path.Join(basePath, name)
	return p
}

func (t *BashTarget) AddResource(resource Resource) (string, error) {
	dynamicResource, ok := resource.(DynamicResource)
	if ok {
		path := t.generateDynamicPath(dynamicResource.Prefix())
		f, err := os.Create(path)
		if err != nil {
			return "", err
		}
		defer func() {
			err := f.Close()
			if err != nil {
				glog.Warning("Error closing resource file", err)
			}
		}()

		err = dynamicResource.Write(f)
		if err != nil {
			return "", fmt.Errorf("error writing resource: %v", err)
		}

		return path, nil
	}

	switch r := resource.(type) {
	case *FileResource:
		return r.Path, nil
	default:
		log.Fatal("unknown resource type: ", r)
		return "", fmt.Errorf("unknown resource type: %v", r)
	}
}

func (k *SSHKey) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	request := &ec2.DescribeKeyPairsInput{
		KeyNames: []*string{
			aws.String(k.Name),
		},
	}

	response, err := cloud.ec2.DescribeKeyPairs(request)
	if awsErr, ok := err.(awserr.Error); ok {
		if awsErr.Code() == "InvalidKeyPair.NotFound" {
			err = nil
		}
	}
	if err != nil {
		return fmt.Errorf("error listing keys: %v", err)
	}
	found := false
	if response != nil && len(response.KeyPairs) != 0 {
		// TODO: Check key actually matches?
		glog.V(2).Info("found AWS SSH key with name: ", k.Name)
		found = true
	}

	if found {
		return nil
	}

	file, err := output.AddResource(k.PublicKey)
	if err != nil {
		return err
	}
	output.AddEC2Command("import-key-pair", "--key-name", k.Name, "--public-key-material", "file://"+file)
	return nil
}

var basePath string

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

func main() {
	var config Configuration
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
	flag.StringVar(&config.MasterIPRange, "master-ip-range", "10.246.0.0/24", "IP range for master in-cluster (pod) IPs")

	flag.StringVar(&config.DockerStorage, "docker-storage", "aufs", "Filesystem to use for docker storage")

	flag.StringVar(&config.ClusterID, "cluster-id", "", "cluster id")
	flag.StringVar(&volumeType, "volume-type", "gp2", "Type for EBS volumes")
	flag.IntVar(&masterVolumeSize, "master-volume-size", 20, "Size for master volume")
	flag.IntVar(&minionCount, "minion-count", 2, "Number of minions")

	flag.Set("alsologtostderr", "true")

	flag.Parse()

	clusterID := config.ClusterID
	if clusterID == "" {
		glog.Fatal("cluster-id is required")
	}

	az := config.Zone
	if len(az) <= 2 {
		glog.Fatal("Invalid AZ: ", az)
	}
	region := az[:len(az)-1]

	if s3BucketName == "" {
		// TODO: Implement the generation logic
		glog.Fatal("s3-bucket is required (for now!)")
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

	config.KubeUser = "admin"
	config.KubePassword = randomToken(16)

	config.KubeletToken = randomToken(32)
	config.KubeProxyToken = randomToken(32)

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

	distro := &DistroVivid{}
	imageID := distro.GetImageID(region)

	if imageID == "" {
		log.Fatal("ImageID could not be determined")
	}

	instanceType := "m3.medium"

	tags := map[string]string{"KubernetesCluster": clusterID}
	cloud := NewAWSCloud(region, tags)

	target := NewBashTarget(cloud)

	s3Bucket := &S3Bucket{
		Name:         s3BucketName,
		CreateRegion: s3Region,
	}

	s3KubernetesFile := &S3File{
		Bucket: s3Bucket,
		Key:    "devel/kubernetes-server-linux-amd64.tar.gz",
		Source: findKubernetesTarGz(),
		Public: true,
	}

	s3SaltFile := &S3File{
		Bucket: s3Bucket,
		Key:    "devel/kubernetes-salt.tar.gz",
		Source: findSaltTarGz(),
		Public: true,
	}

	s3Resources := []BashRenderable{
		s3Bucket,
		s3KubernetesFile,
		s3SaltFile,
	}

	glog.Info("Processing S3 resources")
	renderItems(s3Resources, cloud, target)

	config.ServerBinaryTarURL = s3KubernetesFile.PublicURL()
	config.SaltTarURL = s3SaltFile.PublicURL()

	iamMasterRole := &IAMRole{
		Name:               "kubernetes-master",
		RolePolicyDocument: staticResource("cluster/aws/templates/iam/kubernetes-master-role.json"),
	}
	iamMasterRolePolicy := &IAMRolePolicy{
		Role:           iamMasterRole,
		Name:           "kubernetes-master",
		PolicyDocument: staticResource("cluster/aws/templates/iam/kubernetes-master-policy.json"),
	}
	iamMasterInstanceProfile := &IAMInstanceProfile{
		Name: "kubernetes-master",
		Role: iamMasterRole,
	}

	iamMinionRole := &IAMRole{
		Name:               "kubernetes-minion",
		RolePolicyDocument: staticResource("cluster/aws/templates/iam/kubernetes-minion-role.json"),
	}
	iamMinionRolePolicy := &IAMRolePolicy{
		Role:           iamMinionRole,
		Name:           "kubernetes-minion",
		PolicyDocument: staticResource("cluster/aws/templates/iam/kubernetes-minion-policy.json"),
	}
	iamMinionInstanceProfile := &IAMInstanceProfile{
		Name: "kubernetes-minion",
		Role: iamMinionRole,
	}

	sshKey := &SSHKey{Name: "kubernetes-" + clusterID, PublicKey: &FileResource{Path: "~/.ssh/justin2015.pub"}}
	vpc := &VPC{CIDR: "172.20.0.0/16"}
	subnet := &Subnet{VPC: vpc, AZ: az, CIDR: "172.20.0.0/24"}
	igw := &InternetGateway{VPC: vpc}
	routeTable := &RouteTable{Subnet: subnet}
	route := &Route{RouteTable: routeTable, CIDR: "0.0.0.0/0", InternetGateway: igw}
	masterSG := &SecurityGroup{
		Name:        "kubernetes-master-" + clusterID,
		Description: "Security group for master nodes",
		VPC:         vpc}
	minionSG := &SecurityGroup{
		Name:        "kubernetes-minion-" + clusterID,
		Description: "Security group for minion nodes",
		VPC:         vpc}

	masterPV := &PersistentVolume{
		AZ:         az,
		Size:       masterVolumeSize,
		VolumeType: volumeType,
		NameTag:    clusterID + "-master-pd",
	}

	masterUserData := &MasterScript{
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
	minionUserData := &MinionScript{
		Config: &config,
	}

	masterInstance := &Instance{
		NameTag: clusterID + "-master",
		InstanceConfig: InstanceConfig{
			Subnet:              subnet,
			SSHKey:              sshKey,
			SecurityGroups:      []*SecurityGroup{masterSG},
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

	minionConfiguration := &AutoscalingLaunchConfiguration{
		Name: clusterID + "-minion-group",
		InstanceConfig: InstanceConfig{
			SSHKey:              sshKey,
			SecurityGroups:      []*SecurityGroup{minionSG},
			IAMInstanceProfile:  iamMinionInstanceProfile,
			ImageID:             imageID,
			InstanceType:        instanceType,
			AssociatePublicIP:   true,
			BlockDeviceMappings: minionBlockDeviceMappings,
			UserData:            minionUserData,
		},
	}

	minionGroup := &AutoscalingGroup{
		Name:                clusterID + "-minion-group",
		LaunchConfiguration: minionConfiguration,
		MinSize:             minionCount,
		MaxSize:             minionCount,
		Subnet:              subnet,
		Tags: map[string]string{
			"Role": "minion",
		},
	}

	resources := []BashRenderable{
		iamMasterRole, iamMasterRolePolicy, iamMasterInstanceProfile,
		iamMinionRole, iamMinionRolePolicy, iamMinionInstanceProfile,
		sshKey, vpc, subnet, igw,
		routeTable, route,
		masterSG, minionSG,
		masterPV,
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
	target.DebugDump()

	err = target.PrintShellCommands(os.Stdout)
	if err != nil {
		glog.Fatal("error building shell commands: %v", err)
	}
}
func renderItems(items []BashRenderable, cloud *AWSCloud, target *BashTarget) {
	for _, resource := range items {
		glog.Info("rendering ", resource)
		err := resource.RenderBash(cloud, target)
		if err != nil {
			glog.Fatalf("error rendering resource (%v): %v", err)
		}
	}
}
