package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

type SubnetRenderer interface {
	RenderSubnet(actual, expected, changes *Subnet) error
}

type Subnet struct {
	ID               *string
	VPC              *VPC
	VPCID            *string
	AvailabilityZone *string
	CIDR             *string
}

func (s *Subnet) Prefix() string {
	return "Subnet"
}

func (e *Subnet) find(c *Context) (*Subnet, error) {
	cloud := c.Cloud

	actual := &Subnet{}
	request := &ec2.DescribeSubnetsInput{
		Filters: cloud.BuildFilters(),
	}

	response, err := cloud.EC2.DescribeSubnets(request)
	if err != nil {
		return nil, fmt.Errorf("error listing Subnets: %v", err)
	}
	if response == nil || len(response.Subnets) == 0 {
		return nil, nil
	} else {
		if len(response.Subnets) != 1 {
			glog.Fatalf("found multiple Subnets matching tags")
		}
		subnet := response.Subnets[0]
		actual.ID = subnet.SubnetId
		actual.AvailabilityZone = subnet.AvailabilityZone
		actual.VPCID = subnet.VpcId
		glog.V(2).Infof("found matching subnet %q", *actual.ID)
	}

	return actual, nil
}

func (e *Subnet) Run(c *Context) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &Subnet{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	err = e.checkChanges(a, e, changes)
	if err != nil {
		return err
	}

	target := c.Target.(SubnetRenderer)
	return target.RenderSubnet(a, e, changes)
}

func (s *Subnet) checkChanges(a, e, changes *Subnet) error {
	if a != nil {
		if changes.VPCID != nil {
			// TODO: Do we want to destroy & recreate the CIDR?
			return InvalidChangeError("Cannot change subnet VPC", changes.VPCID, e.VPCID)
		}
		if changes.AvailabilityZone != nil {
			// TODO: Do we want to destroy & recreate the CIDR?
			return InvalidChangeError("Cannot change subnet AvailabilityZone", changes.AvailabilityZone, e.AvailabilityZone)
		}
		if changes.CIDR != nil {
			// TODO: Do we want to destroy & recreate the CIDR?
			return InvalidChangeError("Cannot change subnet CIDR", changes.CIDR, e.CIDR)
		}
	}
	return nil
}

func (t *AWSAPITarget) RenderSubnet(a, e, changes *Subnet) error {
	if a == nil {
		if e.CIDR == nil {
			// TODO: Auto-assign CIDR
			return MissingValueError("Must specify CIDR for Subnet create")
		}

		glog.V(2).Infof("Creating Subnet with CIDR: %q", *e.CIDR)

		vpcID := e.VPCID
		if vpcID == nil && e.VPC != nil {
			vpcID = e.VPC.ID
		}

		request := &ec2.CreateSubnetInput{}
		request.CidrBlock = e.CIDR
		request.AvailabilityZone = e.AvailabilityZone
		request.VpcId = vpcID

		response, err := t.cloud.EC2.CreateSubnet(request)
		if err != nil {
			return fmt.Errorf("error creating subnet: %v", err)
		}

		subnet := response.Subnet
		e.ID = subnet.SubnetId
	}

	return nil //return output.AddAWSTags(cloud.Tags(), v, "vpc")
}

func (t *BashTarget) RenderSubnet(a, e, changes *Subnet) error {
	t.CreateVar(e)
	if a == nil {
		if e.CIDR == nil {
			// TODO: Auto-assign CIDR
			return MissingValueError("Must specify CIDR for Subnet create")
		}

		vpcID := StringValue(e.VPCID)
		if vpcID == "" {
			vpcID = t.ReadVar(e.VPC)
		}

		args := []string{"create-subnet", "--cidr-block", *e.CIDR, "--vpc-id", vpcID, "--query", "Subnet.SubnetId"}
		if e.AvailabilityZone != nil {
			args = append(args, "--availability-zone", *e.AvailabilityZone)
		}

		t.AddEC2Command(args...).AssignTo(e)
	} else {
		t.AddAssignment(e, StringValue(a.ID))
	}

	return nil
	//return t.AddAWSTags(t.cloud.Tags(), e, "vpc")
}
