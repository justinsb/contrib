package kup

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"k8s.io/contrib/installer/pkg/fi"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/golang/glog"
)

type VPC struct {
	CIDR              string
	EnableDNSHostname *bool
	EnableDNSSupport  *bool
}

func (e *VPC) Run(c *fi.RunContext) error {
	cloud := c.Cloud().(*fi.AWSCloud)
	var vpc *ec2.Vpc

	{
		request := &ec2.DescribeVpcsInput{
			Filters: cloud.BuildFilters(),
		}

		response, err := cloud.EC2.DescribeVpcs(request)
		if err != nil {
			return fmt.Errorf("error listing VPCs: %v", err)
		}

		if response != nil && len(response.Vpcs) != 0 {
			if len(response.Vpcs) != 1 {
				glog.Fatalf("found multiple VPCs matching tags")
			}
			glog.V(2).Info("found matching VPC")
			vpc = response.Vpcs[0]
		}
	}

	vpcID := ""
	if vpc != nil {
		vpcID = *vpc.VpcId
		glog.V(2).Infof("Found existing VPC: %q", vpcID)

		// TODO: Do we want to destroy & recreate the CIDR?
		if v.CIDR != aws.StringValue(vpc.CidrBlock) {
			return fmt.Errorf("VPC did not hve the correct CIDR: %q vs %q", v.CIDR, aws.StringValue(vpc.CidrBlock))
		}
	}

	if c.IsConfigure() {
		glog.V(2).Infof("Creating VPC with CIDR: %q", m.CIDR)

		request := &ec2.CreateVpcInput{}
		request.CidrBlock = aws.String(v.CIDR)

		response, err := cloud.EC2.CreateVpc(request)
		if err != nil {
			return fmt.Errorf("error creating VPC: %v", err)
		}

		vpc = response.Vpc
		vpcID = *vpc.VpcId
	} else if c.IsValidate() {
		if vpc == nil {
			c.MarkDirty()
			return nil
		}
	} else {
		panic("Unhandled RunMode")
	}

	if v.EnableDnsSupport != nil {
		request := &ec2.DescribeVpcAttributeInput{VpcId: &vpcID, Attribute: aws.String(ec2.VpcAttributeNameEnableDnsSupport)}
		response, err := cloud.EC2.DescribeVpcAttribute(request)
		if err != nil {
			return fmt.Errorf("error querying for dns support: %v", err)
		}

		if *v.EnableDnsSupport != *response.EnableDnsSupport.Value {
			request := &ec2.ModifyVpcAttributeInput{}
			request.EnableDnsSupport = v.EnableDnsSupport

			_, err := cloud.EC2.ModifyVpcAttribute(request)
			if err != nil {
				return fmt.Errorf("error modifying VPC attribute: %v", err)
			}
		}
	}

	if v.EnableDnsHostnames != nil {
		request := &ec2.DescribeVpcAttributeInput{VpcId: existing.VpcId, Attribute: aws.String(ec2.VpcAttributeNameEnableDnsHostnames)}
		response, err := cloud.EC2.DescribeVpcAttribute(request)
		if err != nil {
			return fmt.Errorf("error querying for dns hostnames: %v", err)
		}
		if *v.EnableDnsHostnames != *response.EnableDnsHostnames.Value {
		request := &ec2.ModifyVpcAttributeInput{}
		request.EnableDnsHostnames = *v.EnableDnsHostnames

		_, err := cloud.EC2.ModifyVpcAttribute(request)
		if err != nil {
			return fmt.Errorf("error modifying VPC attribute: %v", err)
		}
	}

	return output.AddAWSTags(cloud.Tags(), v, "vpc")
}


func newEc2Filter(name string, values ...string) *ec2.Filter {
	awsValues := []*string{}
	for _, value := range values {
		awsValues = append(awsValues, aws.String(value))
	}
	filter := &ec2.Filter{
		Name:   aws.String(name),
		Values: awsValues,
	}
	return filter
}

func (c *AWSCloud) Tags() map[string]string {
	// Defensive copy
	tags := make(map[string]string)
	for k, v := range c.tags {
		tags[k] = v
	}
	return tags
}

func (c *AWSCloud) GetTags(resourceId string, resourceType string) (map[string]string, error) {
	tags := map[string]string{}

	request := &ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			newEc2Filter("resource-id", resourceId),
			newEc2Filter("resource-type", resourceType),
		},
	}

	response, err := c.ec2.DescribeTags(request)
	if err != nil {
		return nil, fmt.Errorf("error listing tags on %v:%v: %v", resourceType, resourceId, err)
	}

	for _, tag := range response.Tags {
		if tag == nil {
			glog.Warning("unexpected nil tag")
			continue
		}
		tags[aws.StringValue(tag.Key)] = aws.StringValue(tag.Value)
	}

	return tags, nil
}

func (c *AWSCloud) BuildFilters() []*ec2.Filter {
	filters := []*ec2.Filter{}
	for name, value := range c.tags {
		filter := newEc2Filter("tag:"+name, value)
		filters = append(filters, filter)
	}
	return filters
}
