package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/golang/glog"
)

var templateDir = "templates"

type BashRenderable interface {
	RenderBash(cloud *AWSCloud, output *BashTarget) error
}

type SSHKey struct {
	Name          string
	PublicKeyPath string
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

type ASGLaunchConfiguration struct {
	name               string
	imageID            string
	iamInstanceProfile string
	instanceType       string
	sshKey             *SSHKey
	securityGroups     []*SecurityGroup
	publicIP           bool
	userData           string
}

type AutoScalingGroup struct {
	name            string
	launchConfigure *ASGLaunchConfiguration
	minSize         int
	maxSize         int
	subnet          *Subnet
}

type AWSCloud struct {
	region string
	iam    *iam.IAM
	ec2    *ec2.EC2
	tags   map[string]string
}

func NewAWSCloud(region string, tags map[string]string) *AWSCloud {
	c := &AWSCloud{region: region}
	config := aws.NewConfig().WithRegion(region)
	c.ec2 = ec2.New(config)
	c.iam = iam.New(config)
	c.tags = tags
	return c
}

func newEc2Filter(name string, value string) *ec2.Filter {
	filter := &ec2.Filter{
		Name: aws.String(name),
		Values: []*string{
			aws.String(value),
		},
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
	iamArgs              []string
	vars                 map[HasId]*BashVar
	prefixCounts         map[string]int
	resourcePrefixCounts map[string]int
}

func NewBashTarget(cloud *AWSCloud) *BashTarget {
	b := &BashTarget{cloud: cloud}
	b.ec2Args = []string{"aws", "ec2"}
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
	default:
		log.Fatal("unknown resource type: ", r)
		return "", fmt.Errorf("unknown resource type: %v", r)
	case FileResource:
		return r.Path, nil
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

	file, err := output.AddResource(k.PublicKeyPath)
	if err != nil {
		return err
	}
	output.AddEC2Command("import-key-pair", "--key-name", k.Name, "--public-key-material", file)
	return nil
}

var basePath string

func staticResource(key string) Resource {
	p := path.Join(basePath, key)
	return &FileResource{Path: p}
}

func main() {
	var config Configuration
	var clusterID string
	var masterVolumeSize int
	var volumeType string

	basePath = "/Users/justinsb/k8s/src/github.com/GoogleCloudPlatform/kubernetes/"

	flag.StringVar(&clusterID, "cluster-id", "", "cluster id")
	flag.StringVar(&config.Zone, "az", "us-east-1b", "AWS availability zone")
	flag.StringVar(&volumeType, "volume-type", "gp2", "Type for EBS volumes")
	flag.IntVar(&masterVolumeSize, "master-volume-size", 20, "Size for master volume")

	flag.Set("alsologtostderr", "true")

	flag.Parse()

	if clusterID == "" {
		glog.Fatal("cluster-id is required")
	}

	az := config.Zone
	if len(az) <= 2 {
		glog.Fatal("Invalid AZ: ", az)
	}
	region := az[:len(az)-1]

	distro := &DistroVivid{}
	imageID := distro.GetImageID(region)

	if imageID == "" {
		log.Fatal("ImageID could not be determined")
	}

	instanceType := "m3.medium"

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

	sshKey := &SSHKey{Name: "kubernetes-" + clusterID, PublicKeyPath: "~/.ssh/justin2015.pub"}
	vpc := &VPC{CIDR: "172.20.0.0/16"}
	subnet := &Subnet{VPC: vpc, AZ: az, CIDR: "172.20.0.0/24"}
	masterInternalIP := "172.20.0.9"
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

	masterInstance := &Instance{
		NameTag:             clusterID + "-master",
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
		Tags:                map[string]string{"Role": "master"},
	}

	resources := []BashRenderable{
		iamMasterRole, iamMasterRolePolicy, iamMasterInstanceProfile,
		iamMinionRole, iamMinionRolePolicy, iamMinionInstanceProfile,
		sshKey, vpc, subnet, igw,
		routeTable, route,
		masterSG, minionSG,
		masterPV,
		masterInstance,
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

	tags := map[string]string{"KubernetesCluster": clusterID}
	cloud := NewAWSCloud(region, tags)

	target := NewBashTarget(cloud)

	glog.Info("Starting")
	for _, resource := range resources {
		glog.Info("rendering ", resource)
		err := resource.RenderBash(cloud, target)
		if err != nil {
			glog.Fatalf("error rendering resource (%v): %v", err)
		}
	}

	target.DebugDump()

	err := target.PrintShellCommands(os.Stdout)
	if err != nil {
		glog.Fatal("error building shell commands: %v", err)
	}
}
