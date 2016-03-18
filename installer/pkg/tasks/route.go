package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/fi"
)

type RouteRenderer interface {
	RenderRoute(actual, expected, changes *Route) error
}

type Route struct {
	fi.SimpleUnit

	RouteTable      *RouteTable
	InternetGateway *InternetGateway
	CIDR            *string
}

func (r *Route) Prefix() string {
	return "Route"
}

func (e *Route) find(c *fi.RunContext) (*Route, error) {
	cloud := c.Cloud().(*fi.AWSCloud)

	var routeTableID *string
	if e.RouteTable != nil {
		routeTableID = e.RouteTable.ID
	}

	cidr := e.CIDR

	if routeTableID == nil || cidr == nil {
		return nil, nil
	}

	filters := cloud.BuildFilters()
	request := &ec2.DescribeRouteTablesInput{
		RouteTableIds: []*string{routeTableID},
		Filters:       filters,
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
		for _, r := range rt.Routes {
			if aws.StringValue(r.DestinationCidrBlock) != *cidr {
				continue
			}
			actual := &Route{}
			actual.RouteTable = e.RouteTable
			actual.CIDR = r.DestinationCidrBlock
			actual.InternetGateway = e.InternetGateway
			glog.V(2).Infof("found matching Route")
			return actual, nil
		}
	}

	return nil, nil
}

func (e *Route) Run(c *fi.RunContext) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &Route{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	err = e.checkChanges(a, e, changes)
	if err != nil {
		return err
	}

	target := c.Target.(RouteRenderer)
	return target.RenderRoute(a, e, changes)
}

func (s *Route) checkChanges(a, e, changes *Route) error {
	if a != nil {
		if changes.RouteTable != nil {
			return InvalidChangeError("Cannot change Route RouteTable", changes.RouteTable, e.RouteTable)
		}
		if changes.CIDR != nil {
			return InvalidChangeError("Cannot change Route CIDR", changes.CIDR, e.CIDR)
		}
	}
	return nil
}

func (t *AWSAPITarget) RenderRoute(a, e, changes *Route) error {
	if a == nil {
		cidr := e.CIDR
		if cidr == nil {
			return MissingValueError("Must specify CIDR for Route create")
		}

		var igwID *string
		if e.InternetGateway != nil {
			igwID = e.InternetGateway.ID
		}
		if igwID == nil {
			return MissingValueError("Must specify InternetGateway for Route create")
		}

		var routeTableID *string
		if e.RouteTable != nil {
			routeTableID = e.RouteTable.ID
		}
		if routeTableID == nil {
			return MissingValueError("Must specify RouteTable for Route create")
		}

		glog.V(2).Infof("Creating Route with RouteTable:%q CIDR:%q", *routeTableID, *cidr)

		request := &ec2.CreateRouteInput{}
		request.DestinationCidrBlock = cidr
		request.GatewayId = igwID
		request.RouteTableId = routeTableID

		response, err := t.cloud.EC2.CreateRoute(request)
		if err != nil {
			return fmt.Errorf("error creating Route: %v", err)
		}

		if !aws.BoolValue(response.Return) {
			return fmt.Errorf("create Route request failed: %v", response)
		}
	}

	return nil //return output.AddAWSTags(cloud.Tags(), v, "vpc")
}

func (t *BashTarget) RenderRoute(a, e, changes *Route) error {
	//t.CreateVar(e)
	if a == nil {
		cidr := e.CIDR
		if cidr == nil {
			return MissingValueError("Must specify CIDR for Route create")
		}

		t.AddEC2Command("create-route",
			"--route-table-id", t.ReadVar(e.RouteTable),
			"--destination-cidr-block", *cidr,
			"--gateway-id", t.ReadVar(e.InternetGateway))
	} else {
		//t.AddAssignment(e, StringValue(a.ID))
	}

	return nil
	//return output.AddAWSTags(cloud.Tags(), r, "route-table-association")
}