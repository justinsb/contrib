package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

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

	output.AddAWSTags(v, cloud.MissingTags(vpcId, "vpc"))
	return nil
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

	output.AddAWSTags(s, cloud.MissingTags(subnetId, "subnet"))
	return nil
}

type SecurityGroup struct {
	name string
}

type SecurityGroupIngress struct {
	securityGroup *SecurityGroup
	cidr          string
	protocol      string
	port          string
	sourceGroup   *SecurityGroup
}

type PersistentVolume struct {
	name   string
	sizeGB int
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
	ec2  *ec2.EC2
	tags map[string]string
}

func NewAWSCloud(tags map[string]string) *AWSCloud {
	c := &AWSCloud{}
	c.ec2 = ec2.New(nil)
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
	return c.tags
}

func (c *AWSCloud) GetTags(resourceId string, resourceType string) (map[string]string, error) {
	tags := map[string]string{}
	if resourceId == "" {
		return nil, nil
	}

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

func (c *AWSCloud) MissingTags(resourceId string, resourceType string) map[string]string {
	actual, err := c.GetTags(resourceId, resourceType)
	if err != nil {
		glog.Fatal("unexpected error fetching tags for resource: %v", err)
	}
	if actual == nil {
		actual = map[string]string{}
	}

	missing := map[string]string{}
	for k, v := range c.tags {
		actualValue, found := actual[k]
		if found && actualValue == v {
			continue
		}
		missing[k] = v
	}
	return missing
}

func (c *AWSCloud) BuildFilters() []*ec2.Filter {
	filters := []*ec2.Filter{}
	for name, value := range c.tags {
		filter := newEc2Filter("tag:"+name, value)
		filters = append(filters, filter)
	}
	return filters
}

type BashTarget struct {
	commands []*BashCommand

	ec2Args      []string
	vars         map[HasId]*BashVar
	prefixCounts map[string]int
}

func NewBashTarget() *BashTarget {
	b := &BashTarget{}
	b.ec2Args = []string{"aws", "ec2"}
	b.vars = make(map[HasId]*BashVar)
	b.prefixCounts = make(map[string]int)
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

func (t *BashTarget) AddEC2Command(args ...string) *BashCommand {
	cmd := &BashCommand{parent: t}
	cmd.args = t.ec2Args
	cmd.args = append(cmd.args, args...)

	return t.AddCommand(cmd)
}

func bashQuoteString(s string) string {
	// TODO: Escaping
	return "\"" + s + "\""
}

func (t *BashTarget) AddAWSTags(s HasId, tags map[string]string) {
	for name, value := range tags {
		cmd := &BashCommand{}
		cmd.args = []string{"add-tag", t.ReadVar(s), bashQuoteString(name), bashQuoteString(value)}
		t.AddCommand(cmd)
	}
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

func (t *BashTarget) AddResource(path string) string {
	return "file://" + path
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

	file := output.AddResource(k.PublicKeyPath)
	output.AddEC2Command("import-key-pair", "--key-name", k.Name, "--public-key-material", file)
	return nil
}

func main() {
	var clusterId string
	var az string

	flag.StringVar(&clusterId, "cluster-id", "", "cluster id")
	flag.StringVar(&az, "az", "us-east-1a", "AWS availability zone")
	flag.Parse()

	if clusterId == "" {
		glog.Fatal("cluster-id is required")
	}

	sshKey := &SSHKey{Name: "kubernetes-" + clusterId, PublicKeyPath: "~/.ssh/justin2015.pub"}
	vpc := &VPC{CIDR: "127.20.0.0/16"}
	subnet := &Subnet{VPC: vpc, AZ: az, CIDR: "127.20.0.0/24"}
	igw := &InternetGateway{VPC: vpc}
	routeTable := &RouteTable{Subnet: subnet}
	route := &Route{RouteTable: routeTable, CIDR: "0.0.0.0/0", InternetGateway: igw}
	resources := []BashRenderable{sshKey, vpc, subnet, igw, routeTable, route}

	target := NewBashTarget()

	tags := map[string]string{"KubernetesCluster": clusterId}
	cloud := NewAWSCloud(tags)

	glog.Info("Starting")
	for _, resource := range resources {
		glog.Info("rendering ", resource)
		err := resource.RenderBash(cloud, target)
		if err != nil {
			glog.Fatalf("error rendering resource (%v): %v", err)
		}
	}

	target.DebugDump()
}
