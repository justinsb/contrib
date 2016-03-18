package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

type InternetGatewayRenderer interface {
	RenderInternetGateway(actual, expected, changes *InternetGateway) error
}

type InternetGateway struct {
	ID    *string
	VPC   *VPC
	VPCID *string
}

func (s *InternetGateway) Prefix() string {
	return "InternetGateway"
}

func (s *InternetGateway) GetID() *string {
	return s.ID
}

func (e *InternetGateway) find(c *Context) (*InternetGateway, error) {
	vpcID := e.VPCID
	if vpcID == nil && e.VPC != nil {
		vpcID = e.VPC.ID
	}

	if vpcID == nil {
		return nil, nil
	}

	cloud := c.Cloud

	actual := &InternetGateway{}

	filters := cloud.BuildFilters()
	filters = append(filters, newEc2Filter("attachment.vpc-id", *vpcID))
	request := &ec2.DescribeInternetGatewaysInput{
		Filters: filters,
	}

	response, err := cloud.EC2.DescribeInternetGateways(request)
	if err != nil {
		return nil, fmt.Errorf("error listing InternetGateways: %v", err)
	}
	if response == nil || len(response.InternetGateways) == 0 {
		return nil, nil
	} else {
		if len(response.InternetGateways) != 1 {
			glog.Fatalf("found multiple InternetGateways matching tags")
		}
		igw := response.InternetGateways[0]
		actual.ID = igw.InternetGatewayId
		actual.VPCID = vpcID
		glog.V(2).Infof("found matching InternetGateway %q", *actual.ID)
	}

	return actual, nil
}

func (e *InternetGateway) Run(c *Context) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &InternetGateway{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	err = e.checkChanges(a, e, changes)
	if err != nil {
		return err
	}

	target := c.Target.(InternetGatewayRenderer)
	return target.RenderInternetGateway(a, e, changes)
}

func (s *InternetGateway) checkChanges(a, e, changes *InternetGateway) error {
	if a != nil {
		// TODO: I think we can change it; we just detatch & attach
		if changes.VPCID != nil {
			return InvalidChangeError("Cannot change InternetGateway VPC", changes.VPCID, e.VPCID)
		}
	}
	return nil
}

func (t *AWSAPITarget) RenderInternetGateway(a, e, changes *InternetGateway) error {
	if a == nil {
		glog.V(2).Infof("Creating InternetGateway")

		vpcID := e.VPCID
		if vpcID == nil && e.VPC != nil {
			vpcID = e.VPC.ID
		}

		request := &ec2.CreateInternetGatewayInput{}

		response, err := t.cloud.EC2.CreateInternetGateway(request)
		if err != nil {
			return fmt.Errorf("error creating InternetGateway: %v", err)
		}

		igw := response.InternetGateway
		e.ID = igw.InternetGatewayId
	}

	return nil //return output.AddAWSTags(cloud.Tags(), v, "vpc")
}

func (t *BashTarget) RenderInternetGateway(a, e, changes *InternetGateway) error {
	t.CreateVar(e)
	if a == nil {
		vpcID := StringValue(e.VPCID)
		if vpcID == "" {
			vpcID = t.ReadVar(e.VPC)
		}

		t.AddEC2Command("create-internet-gateway", "--query", "InternetGateway.InternetGatewayId").AssignTo(e)
		t.AddEC2Command("attach-internet-gateway", "--internet-gateway-id", t.ReadVar(e), "--vpc-id", vpcID)
	} else {
		t.AddAssignment(e, StringValue(a.ID))
	}

	return nil
	//return output.AddAWSTags(cloud.Tags(), g, "internet-gateway")
}
