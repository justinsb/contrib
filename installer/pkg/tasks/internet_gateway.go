package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/fi"
)

type InternetGatewayRenderer interface {
	RenderInternetGateway(actual, expected, changes *InternetGateway) error
}

type InternetGateway struct {
	fi.SimpleUnit

	Name *string
	ID   *string
}

func (s *InternetGateway) Key() string {
	return *s.Name
}

func (s *InternetGateway) GetID() *string {
	return s.ID
}

func (e *InternetGateway) find(c *fi.RunContext) (*InternetGateway, error) {
	cloud := c.Cloud().(*fi.AWSCloud)

	actual := &InternetGateway{}

	filters := cloud.BuildFilters(e.Name)
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
		glog.V(2).Infof("found matching InternetGateway %q", *actual.ID)
	}

	return actual, nil
}

func (e *InternetGateway) Run(c *fi.RunContext) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	if a != nil && e.ID == nil {
		e.ID = a.ID
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
	}
	return nil
}

func (t *AWSAPITarget) RenderInternetGateway(a, e, changes *InternetGateway) error {
	if a == nil {
		glog.V(2).Infof("Creating InternetGateway")

		request := &ec2.CreateInternetGatewayInput{
		}

		response, err := t.cloud.EC2.CreateInternetGateway(request)
		if err != nil {
			return fmt.Errorf("error creating InternetGateway: %v", err)
		}

		igw := response.InternetGateway
		e.ID = igw.InternetGatewayId
	}

	return t.AddAWSTags(*e.ID, "internet-gateway", t.cloud.BuildTags(e.Name))
}

func (t *BashTarget) RenderInternetGateway(a, e, changes *InternetGateway) error {
	t.CreateVar(e)
	if a == nil {
		t.AddEC2Command("create-internet-gateway", "--query", "InternetGateway.InternetGatewayId").AssignTo(e)
	} else {
		t.AddAssignment(e, StringValue(a.ID))
	}

	return t.AddAWSTags(e, "internet-gateway", t.cloud.BuildTags(e.Name))
}
