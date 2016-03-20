package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/fi"
)

type VPCRenderer interface {
	RenderVPC(actual, expected, changes *VPC) error
}

type VPC struct {
	fi.SimpleUnit

	Name               *string
	ID                 *string
	CIDR               *string
	EnableDNSHostnames *bool
	EnableDNSSupport   *bool
}

func (s *VPC) Key() string {
	return *s.Name
}

func (s *VPC) GetID() *string {
	return s.ID
}

func (v *VPC) String() string {
	s := "VPC{"
	if v.CIDR != nil {
		s = s + "CIDR=" + *v.CIDR + " "
	}
	if v.ID != nil {
		s = s + "ID=" + *v.ID + " "
	}
	if v.EnableDNSHostnames != nil {
		s = s + fmt.Sprintf("EnableDNSHostnames=%v ", *v.EnableDNSHostnames)
	}
	if v.EnableDNSSupport != nil {
		s = s + fmt.Sprintf("EnableDNSSupport=%v ", *v.EnableDNSSupport)
	}
	s = s + "}"
	return s
}

func (e *VPC) find(c *fi.RunContext) (*VPC, error) {
	cloud := c.Cloud().(*fi.AWSCloud)

	request := &ec2.DescribeVpcsInput{
		Filters: cloud.BuildFilters(e.Name),
	}

	response, err := cloud.EC2.DescribeVpcs(request)
	if err != nil {
		return nil, fmt.Errorf("error listing VPCs: %v", err)
	}
	if response == nil || len(response.Vpcs) == 0 {
		return nil, nil
	}

	if len(response.Vpcs) != 1 {
		glog.Fatalf("found multiple VPCs matching tags")
	}
	vpc := response.Vpcs[0]
	actual := &VPC{}
	actual.ID = vpc.VpcId
	actual.CIDR = vpc.CidrBlock
	glog.V(2).Infof("found matching VPC %q", *actual.ID)

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
	glog.V(4).Infof("found matching VPC %v", actual.String())

	return actual, nil
}

func (e *VPC) Run(c *fi.RunContext) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	if a != nil && e.ID == nil {
		e.ID = a.ID
	}

	changes := &VPC{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		glog.V(2).Infof("No changed: %v", e)
		return nil
	}

	target := c.Target.(VPCRenderer)
	return target.RenderVPC(a, e, changes)
}

func (t *AWSAPITarget) RenderVPC(a, e, changes *VPC) error {
	glog.V(2).Infof("Rendering via AWS API: %v", changes)
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

		e.ID = response.Vpc.VpcId
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

	return t.AddAWSTags(*e.ID, "vpc", t.cloud.BuildTags(e.Name))
}

func (t *BashTarget) RenderVPC(a, e, changes *VPC) error {
	glog.V(2).Infof("Rendering to bash: %v", changes)
	t.CreateVar(e)

	if a == nil {
		if e.CIDR == nil {
			// TODO: Auto-assign CIDR
			return MissingValueError("Must specify CIDR for VPC create")
		}

		glog.V(2).Infof("Creating VPC with CIDR: %q", *e.CIDR)

		t.AddEC2Command("create-vpc", "--cidr-block", *e.CIDR, "--query", "Vpc.VpcId").AssignTo(e)
	} else {
		if changes.CIDR != nil {
			// TODO: Do we want to destroy & recreate the CIDR?
			return InvalidChangeError("VPC did not have the correct CIDR", changes.CIDR, e.CIDR)
		}

		t.AddAssignment(e, StringValue(a.ID))
	}

	if changes.EnableDNSSupport != nil {
		s := fmt.Sprintf("'{\"Value\": %v}'", *changes.EnableDNSSupport)
		t.AddEC2Command("modify-vpc-attribute", "--vpc-id", t.ReadVar(e), "--enable-dns-support", s)
	}

	if changes.EnableDNSHostnames != nil {
		s := fmt.Sprintf("'{\"Value\": %v}'", *changes.EnableDNSHostnames)
		t.AddEC2Command("modify-vpc-attribute", "--vpc-id", t.ReadVar(e), "--enable-dns-hostnames", s)
	}

	return t.AddAWSTags(e, "vpc", t.cloud.BuildTags(e.Name))
}
