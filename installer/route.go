package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

type Route struct {
	RouteTable      *RouteTable
	InternetGateway *InternetGateway
	CIDR            string
}

func (r *Route) Prefix() string {
	return "Route"
}

func (r *Route) String() string {
	return fmt.Sprintf("Route (CIDR=%s)", r.CIDR)
}

func (r *Route) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	routeTableId, _ := output.FindValue(r.RouteTable)
	igwId, _ := output.FindValue(r.InternetGateway)

	var existingRoute *ec2.Route
	if routeTableId != "" && igwId != "" {
		request := &ec2.DescribeRouteTablesInput{
			Filters: []*ec2.Filter{newEc2Filter("route-table-id", routeTableId)},
		}

		response, err := cloud.ec2.DescribeRouteTables(request)
		if err != nil {
			return fmt.Errorf("error listing route tables: %v", err)
		}

		var existing *ec2.RouteTable
		if response != nil && len(response.RouteTables) != 0 {
			if len(response.RouteTables) != 1 {
				glog.Fatalf("found multiple route tables for id: %s", routeTableId)
			}
			glog.V(2).Info("found existing route table")
			existing = response.RouteTables[0]
		}

		if existing != nil {
			for _, route := range existing.Routes {
				if aws.StringValue(route.GatewayId) != igwId {
					continue
				}
				if aws.StringValue(route.DestinationCidrBlock) != r.CIDR {
					continue
				}
				existingRoute = route
				break
			}
		}
	}

	if existingRoute == nil {
		glog.V(2).Info("Route not found; will create")
		output.AddEC2Command("create-route",
			"--route-table-id", output.ReadVar(r.RouteTable),
			"--destination-cidr-block", r.CIDR,
			"--gateway-id", output.ReadVar(r.InternetGateway))
	}

	return nil
}
