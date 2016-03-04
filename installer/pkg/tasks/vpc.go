package tasks

import (
	"fmt"
	"reflect"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

type VPCRenderer interface {
	RenderVPC(actual, expected, changes *VPC) error
}

type VPC struct {
	ID                 *string
	CIDR               *string
	EnableDNSHostnames *bool
	EnableDNSSupport   *bool
}

func (e *VPC) find(c *Context) (*VPC, error) {
	cloud := c.Cloud

	actual := &VPC{}
	request := &ec2.DescribeVpcsInput{
	//Filters: cloud.BuildFilters(),
	}

	response, err := cloud.EC2.DescribeVpcs(request)
	if err != nil {
		return nil, fmt.Errorf("error listing VPCs: %v", err)
	}
	if response == nil || len(response.Vpcs) == 0 {
		return nil, nil
	} else {
		if len(response.Vpcs) != 1 {
			glog.Fatalf("found multiple VPCs matching tags")
		}
		vpc := response.Vpcs[0]
		actual.ID = vpc.VpcId
		glog.V(2).Infof("found matching VPC %q", *actual.ID)
	}

	if actual.ID != nil {
		request := &ec2.DescribeVpcAttributeInput{VpcId: actual.ID, Attribute: aws.String(ec2.VpcAttributeNameEnableDnsSupport)}
		response, err := cloud.EC2.DescribeVpcAttribute(request)
		if err != nil {
			return nil, fmt.Errorf("error querying for dns support: %v", err)
		}
		actual.EnableDNSSupport = response.EnableDnsSupport.Value
	}

	if actual.ID != nil {
		request := &ec2.DescribeVpcAttributeInput{VpcId: actual.ID, Attribute: aws.String(ec2.VpcAttributeNameEnableDnsHostnames)}
		response, err := cloud.EC2.DescribeVpcAttribute(request)
		if err != nil {
			return nil, fmt.Errorf("error querying for dns support: %v", err)
		}
		actual.EnableDNSHostnames = response.EnableDnsHostnames.Value
	}

	return actual, nil
}

func BuildChanges(a, e, changes interface{}) bool {
	changed := false

	va := reflect.ValueOf(a)
	ve := reflect.ValueOf(e)
	vc := reflect.ValueOf(changes)

	va = va.Elem()
	ve = ve.Elem()
	vc = vc.Elem()

	t := va.Type()
	if t != ve.Type() {
		panic("mismatched types in BuildChanges")
	}

	for i := 0; i < va.NumField(); i++ {
		fva := va.Field(i)
		fve := ve.Field(i)

		if fve.IsNil() {
			// No expected value means 'don't change'
			continue
		}

		if reflect.DeepEqual(fva.Interface(), fve.Interface()) {
			continue
		}

		glog.V(8).Infof("Field changed %q %q %q", t.Field(i).Name, fva.Interface(), fve.Interface())
		changed = true
		vc.Field(i).Set(fva)
	}

	return changed
}

func (e *VPC) Run(c *Context) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &VPC{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	target := c.Target.(VPCRenderer)
	return target.RenderVPC(a, e, changes)
}

func (v *VPC) Prefix() string {
	return "Vpc"
}

func (v *VPC) String() string {
	return fmt.Sprintf("VPC (cidr=%s)", v.CIDR)
}

type AWSAPITarget struct {
	cloud *AWSCloud
}

func StringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func MissingValueError(message string) error {
	return fmt.Errorf("%s", message)
}

func InvalidChangeError(message string, actual, expected interface{}) error {
	return fmt.Errorf("%s current=%q, desired=%q", actual, expected)
}

func (t *AWSAPITarget) RenderVPC(a, e, changes *VPC) error {
	id := StringValue(e.ID)
	if changes.CIDR != nil {
		// TODO: Do we want to destroy & recreate the CIDR?
		return InvalidChangeError("VPC did not have the correct CIDR", changes.CIDR, e.CIDR)
	}

	if id == "" {
		if e.CIDR == nil {
			// TODO: Auto-assign CIDR
			return MissingValueError("Must specify CIDR for VPC create")
		}

		glog.V(2).Infof("Creating VPC with CIDR: %q", *e.CIDR)

		request := &ec2.CreateVpcInput{}
		request.CidrBlock = e.CIDR

		response, err := t.cloud.EC2.CreateVpc(request)
		if err != nil {
			return fmt.Errorf("error creating VPC: %v", err)
		}

		vpc := response.Vpc
		id = *vpc.VpcId
	}

	if changes.EnableDNSSupport != nil {
		request := &ec2.ModifyVpcAttributeInput{}
		request.VpcId = aws.String(id)
		request.EnableDnsSupport = &ec2.AttributeBooleanValue{Value: changes.EnableDNSSupport}

		_, err := t.cloud.EC2.ModifyVpcAttribute(request)
		if err != nil {
			return fmt.Errorf("error modifying VPC attribute: %v", err)
		}
	}

	if changes.EnableDNSHostnames != nil {
		request := &ec2.ModifyVpcAttributeInput{}
		request.VpcId = aws.String(id)
		request.EnableDnsHostnames = &ec2.AttributeBooleanValue{Value: changes.EnableDNSHostnames}

		_, err := t.cloud.EC2.ModifyVpcAttribute(request)
		if err != nil {
			return fmt.Errorf("error modifying VPC attribute: %v", err)
		}
	}

	return nil //return output.AddAWSTags(cloud.Tags(), v, "vpc")
}

func (t *BashTarget) RenderVPC(a, e, changes *VPC) error {
	t.CreateVar(e)

	if changes.CIDR != nil {
		// TODO: Do we want to destroy & recreate the CIDR?
		return InvalidChangeError("VPC did not have the correct CIDR", changes.CIDR, e.CIDR)
	}

	if StringValue(a.ID) == "" {
		if e.CIDR == nil {
			// TODO: Auto-assign CIDR
			return MissingValueError("Must specify CIDR for VPC create")
		}

		t.AddEC2Command("create-vpc", "--cidr-block", *e.CIDR, "--query", "Vpc.VpcId").AssignTo(e)
	} else {
		t.AddAssignment(e, StringValue(a.ID))
	}

	if changes.EnableDNSSupport != nil {
		s := fmt.Sprintf("'{\"Value\": %v}'", *changes.EnableDNSSupport)
		t.AddEC2Command("modify-vpc-attribute", "--vpc-id", t.ReadVar(e), "--enable-dns-support", s)
	}

	if changes.EnableDNSHostnames != nil {
		s := fmt.Sprintf("'{\"Value\": %v}'", *changes.EnableDNSSupport)
		t.AddEC2Command("modify-vpc-attribute", "--vpc-id", t.ReadVar(e), "--enable-dns-hostnames", s)
	}

	return t.AddAWSTags(t.cloud.Tags(), e, "vpc")
}
