package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

type RouteTable struct {
	Subnet *Subnet
}

func (r *RouteTable) Prefix() string {
	return "RouteTable"
}

func (r *RouteTable) String() string {
	return fmt.Sprintf("RouteTable (subnet=%s)", r.Subnet)
}

func (r *RouteTable) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	subnetId, _ := output.FindValue(r.Subnet)

	routeTableId := ""
	if subnetId != "" {
		request := &ec2.DescribeRouteTablesInput{
			Filters: []*ec2.Filter{newEc2Filter("association.subnet-id", subnetId)},
		}

		response, err := cloud.ec2.DescribeRouteTables(request)
		if err != nil {
			return fmt.Errorf("error listing route tables: %v", err)
		}

		var existing *ec2.RouteTable
		if response != nil && len(response.RouteTables) != 0 {
			if len(response.RouteTables) != 1 {
				glog.Fatalf("found multiple route tables for subnet: %s", subnetId)
			}
			glog.V(2).Info("found existing route table")
			existing = response.RouteTables[0]
			routeTableId = aws.StringValue(existing.RouteTableId)
		}
	}

	output.CreateVar(r)
	if routeTableId == "" {
		glog.V(2).Info("Route table not found; will create")
		output.AddEC2Command("create-route-table", "--vpc-id", output.ReadVar(r.Subnet.VPC), "--query", "RouteTable.RouteTableId").AssignTo(r)
		output.AddEC2Command("associate-route-table", "--route-table-id", output.ReadVar(r), "--subnet-id", output.ReadVar(r.Subnet))
	} else {
		output.AddAssignment(r, routeTableId)
	}

	return output.AddAWSTags(cloud, r, "route-table")
}
