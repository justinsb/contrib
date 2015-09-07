package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

type InternetGateway struct {
	VPC *VPC
}

func (g *InternetGateway) Prefix() string {
	return "IGW"
}

func (g *InternetGateway) String() string {
	return fmt.Sprintf("Internet Gateway (vpc=%s)", g.VPC.CIDR)
}

func (g *InternetGateway) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	vpcId, _ := output.FindValue(g.VPC)

	igwId := ""
	if vpcId != "" {
		request := &ec2.DescribeInternetGatewaysInput{
			Filters: []*ec2.Filter{newEc2Filter("attachment.vpc-id", vpcId)},
		}

		response, err := cloud.ec2.DescribeInternetGateways(request)
		if err != nil {
			return fmt.Errorf("error listing internet gateways: %v", err)
		}

		var existing *ec2.InternetGateway
		if response != nil && len(response.InternetGateways) != 0 {
			if len(response.InternetGateways) != 1 {
				glog.Fatalf("found multiple internet gateways for vpc: %s", vpcId)
			}
			glog.V(2).Info("found existing internet gateway")
			existing = response.InternetGateways[0]
			igwId = aws.StringValue(existing.InternetGatewayId)
		}
	}

	output.CreateVar(g)
	if igwId == "" {
		glog.V(2).Info("Internet gateway not found; will create")
		output.AddEC2Command("create-internet-gateway", "--query", "InternetGateway.InternetGatewayId").AssignTo(g)
		output.AddEC2Command("attach-internet-gateway", "--internet-gateway-id", output.ReadVar(g), "--vpc-id", output.ReadVar(g.VPC))
	} else {
		output.AddAssignment(g, igwId)
	}

	return output.AddAWSTags(cloud.Tags(), g, "internet-gateway")
}
