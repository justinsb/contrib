package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/fi"
)

type SecurityGroupRenderer interface {
	RenderSecurityGroup(actual, expected, changes *SecurityGroup) error
}

type SecurityGroup struct {
	ID          *string
	Name        *string
	Description *string
	VPC         *VPC
}

func (s *SecurityGroup) Prefix() string {
	return "SecurityGroup"
}

func (s *SecurityGroup) GetID() *string {
	return s.ID
}

func (e *SecurityGroup) find(c *fi.RunContext) (*SecurityGroup, error) {
	cloud := c.Cloud().(*fi.AWSCloud)

	var vpcID *string
	if e.VPC != nil {
		vpcID = e.VPC.ID
	}

	if vpcID == nil || e.Name == nil {
		return nil, nil
	}

	filters := cloud.BuildFilters()
	filters = append(filters, fi.NewEC2Filter("vpc-id", *vpcID))
	filters = append(filters, fi.NewEC2Filter("group-name", *e.Name))

	request := &ec2.DescribeSecurityGroupsInput{
		Filters: filters,
	}

	response, err := cloud.EC2.DescribeSecurityGroups(request)
	if err != nil {
		return nil, fmt.Errorf("error listing SecurityGroups: %v", err)
	}
	if response == nil || len(response.SecurityGroups) == 0 {
		return nil, nil
	} else {
		if len(response.SecurityGroups) != 1 {
			glog.Fatalf("found multiple SecurityGroups matching tags")
		}
		sg := response.SecurityGroups[0]
		actual := &SecurityGroup{}
		actual.ID = sg.GroupId
		glog.V(2).Infof("found matching SecurityGroup %q", *actual.ID)
		return actual, nil
	}

	return nil, nil
}

func (e *SecurityGroup) Run(c *fi.RunContext) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &SecurityGroup{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	err = e.checkChanges(a, e, changes)
	if err != nil {
		return err
	}

	target := c.Target.(SecurityGroupRenderer)
	return target.RenderSecurityGroup(a, e, changes)
}

func (s *SecurityGroup) checkChanges(a, e, changes *SecurityGroup) error {
	if a != nil {
		if changes.ID != nil {
			return InvalidChangeError("Cannot change SecurityGroup ID", changes.ID, e.ID)
		}
		if changes.Name != nil {
			return InvalidChangeError("Cannot change SecurityGroup Name", changes.Name, e.Name)
		}
		if changes.VPC != nil {
			return InvalidChangeError("Cannot change SecurityGroup VPC", changes.VPC, e.VPC)
		}
	}
	return nil
}

func (t *AWSAPITarget) RenderSecurityGroup(a, e, changes *SecurityGroup) error {
	if a == nil {
		vpcID := e.VPC.ID

		glog.V(2).Infof("Creating SecurityGroup with Name:%q VPC:%q", *e.Name, *vpcID)

		request := &ec2.CreateSecurityGroupInput{}
		request.VpcId = vpcID
		request.GroupName = e.Name
		request.Description = e.Description

		response, err := t.cloud.EC2.CreateSecurityGroup(request)
		if err != nil {
			return fmt.Errorf("error creating SecurityGroup: %v", err)
		}

		e.ID = response.GroupId
	}

	return nil //return output.AddAWSTags(cloud.Tags(), v, "vpc")
}

func (t *BashTarget) RenderSecurityGroup(a, e, changes *SecurityGroup) error {
	t.CreateVar(e)
	if a == nil {
		glog.V(2).Infof("Creating SecurityGroup with Name:%q", *e.Name)

		t.AddEC2Command("create-security-group", "--group-name", *e.Name,
			"--description", bashQuoteString(*e.Description),
			"--vpc-id", t.ReadVar(e.VPC),
			"--query", "GroupId").AssignTo(e)
	} else {
		t.AddAssignment(e, StringValue(a.ID))
	}

	return nil
	//return output.AddAWSTags(cloud.Tags(), r, "route-table-association")
}

func (s *SecurityGroup) AllowFrom(source *SecurityGroup) *SecurityGroupIngress {
	return &SecurityGroupIngress{SecurityGroup: s, SourceGroup: source}
}

func (s *SecurityGroup) AllowTCP(cidr string, fromPort int, toPort int) *SecurityGroupIngress {
	fromPort64 := int64(fromPort)
	toPort64 := int64(toPort)
	protocol := "tcp"
	return &SecurityGroupIngress{
		SecurityGroup: s,
		CIDR:          &cidr,
		Protocol:      &protocol,
		FromPort:      &fromPort64,
		ToPort:        &toPort64,
	}
}
