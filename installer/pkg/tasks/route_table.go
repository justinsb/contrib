package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/fi"
)

type RouteTableRenderer interface {
	RenderRouteTable(actual, expected, changes *RouteTable) error
}

type RouteTable struct {
	ID  *string
	VPC *VPC
}

func (s *RouteTable) Prefix() string {
	return "RouteTable"
}

func (s *RouteTable) GetID() *string {
	return s.ID
}

func (e *RouteTable) find(c *fi.RunContext) (*RouteTable, error) {
	cloud := c.Cloud().(*fi.AWSCloud)

	actual := &RouteTable{}
	request := &ec2.DescribeRouteTablesInput{
		Filters: cloud.BuildFilters(),
	}

	response, err := cloud.EC2.DescribeRouteTables(request)
	if err != nil {
		return nil, fmt.Errorf("error listing RouteTables: %v", err)
	}
	if response == nil || len(response.RouteTables) == 0 {
		return nil, nil
	} else {
		if len(response.RouteTables) != 1 {
			glog.Fatalf("found multiple RouteTables matching tags")
		}
		rt := response.RouteTables[0]
		actual.ID = rt.RouteTableId
		actual.VPC = &VPC{ID: rt.VpcId}
		glog.V(2).Infof("found matching RouteTable %q", *actual.ID)
	}

	return actual, nil
}

func (e *RouteTable) Run(c *fi.RunContext) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &RouteTable{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	err = e.checkChanges(a, e, changes)
	if err != nil {
		return err
	}

	target := c.Target.(RouteTableRenderer)
	return target.RenderRouteTable(a, e, changes)
}

func (s *RouteTable) checkChanges(a, e, changes *RouteTable) error {
	if a != nil {
		if changes.VPC != nil && changes.VPC.ID != nil {
			return InvalidChangeError("Cannot change RouteTable VPC", changes.VPC.ID, e.VPC.ID)
		}
	}
	return nil
}

func (t *AWSAPITarget) RenderRouteTable(a, e, changes *RouteTable) error {
	if a == nil {
		vpcID := e.VPC.ID
		if vpcID == nil {
			return MissingValueError("Must specify VPC for RouteTable create")
		}

		glog.V(2).Infof("Creating RouteTable with VPC: %q", *vpcID)

		request := &ec2.CreateRouteTableInput{}
		request.VpcId = vpcID

		response, err := t.cloud.EC2.CreateRouteTable(request)
		if err != nil {
			return fmt.Errorf("error creating RouteTable: %v", err)
		}

		rt := response.RouteTable
		e.ID = rt.RouteTableId
	}

	return nil //return output.AddAWSTags(cloud.Tags(), v, "vpc")
}

func (t *BashTarget) RenderRouteTable(a, e, changes *RouteTable) error {
	t.CreateVar(e)
	if a == nil {
		vpcID := t.ReadVar(e.VPC)

		glog.V(2).Infof("Creating RouteTable with VPC: %q", vpcID)

		t.AddEC2Command("create-route-table", "--vpc-id", vpcID, "--query", "RouteTable.RouteTableId").AssignTo(e)
	} else {
		t.AddAssignment(e, StringValue(a.ID))
	}

	return nil
	//return output.AddAWSTags(cloud.Tags(), r, "route-table")
}
