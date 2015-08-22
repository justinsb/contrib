package main

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

type Instance struct {
	NameTag             string
	ImageID             string
	InstanceType        string
	Subnet              *Subnet
	SSHKey              *SSHKey
	SecurityGroups      []*SecurityGroup
	PrivateIPAddress    string
	AssociatePublicIP   bool
	BlockDeviceMappings []ec2.BlockDeviceMapping
	UserData            Resource
	IAMInstanceProfile  *IAMInstanceProfile
}

func (i *Instance) Prefix() string {
	return "Instance"
}

func (i *Instance) String() string {
	return fmt.Sprintf("Instance (name=%s)", i.NameTag)
}

func (i *Instance) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	var existing *ec2.Instance

	filters := cloud.BuildFilters()
	filters = append(filters, newEc2Filter("tag:Name", i.NameTag))
	request := &ec2.DescribeInstancesInput{
		Filters: filters,
	}

	response, err := cloud.ec2.DescribeInstances(request)
	if err != nil {
		return fmt.Errorf("error listing instances: %v", err)
	}

	if response != nil {
		instances := []*ec2.Instance{}
		for _, reservation := range response.Reservations {
			for _, instance := range reservation.Instances {
				instances = append(instances, instance)
			}
		}

		if len(instances) != 0 {
			if len(instances) != 1 {
				glog.Fatalf("found multiple instances with name: %s", i.NameTag)
			}
			glog.V(2).Info("found existing instance")
			existing = instances[0]
		}
	}

	// TODO: Validate existing

	output.CreateVar(i)
	if existing == nil {
		glog.V(2).Info("instance not found; will create: ", i)
		args := []string{"run-instances"}
		args = append(args, "--image-id", i.ImageID)
		args = append(args, "--instance-type", i.InstanceType)
		args = append(args, "--subnet-id", output.ReadVar(i.Subnet))
		if i.PrivateIPAddress != "" {
			args = append(args, "--private-ip-address", i.PrivateIPAddress)
		}
		if i.SSHKey != nil {
			args = append(args, "--key-name", i.SSHKey.Name)
		}
		if i.SecurityGroups != nil {
			ids := ""
			for _, sg := range i.SecurityGroups {
				if ids != "" {
					ids = ids + ","
				}
				ids = ids + output.ReadVar(sg)
			}
			args = append(args, "--security-group-ids", ids)
		}
		if i.AssociatePublicIP {
			args = append(args, "--associate-public-ip-address")
		} else {
			args = append(args, "--no-associate-public-ip-address")
		}
		if i.BlockDeviceMappings != nil {
			blockDeviceMappings, err := json.Marshal(i.BlockDeviceMappings)
			if err != nil {
				return fmt.Errorf("error converting BlockDeviceMappings to JSON: %v", err)
			}

			args = append(args, "--block-device-mappings", bashQuoteString(string(blockDeviceMappings)))
		}
		if i.UserData != nil {
			tempFile := output.AddResource(i.UserData)
			args = append(args, "--user-data", "file://"+tempFile)
		}
		args = append(args, "--query", "Instances[0].InstanceId")
		if i.IAMInstanceProfile != nil {
			args = append(args, "--iam-instance-profile", "Name="+i.IAMInstanceProfile.Name)
		}

		output.AddEC2Command(args...).AssignTo(i)
	} else {
		output.AddAssignment(i, aws.StringValue(existing.InstanceId))
	}

	tags := cloud.Tags()
	tags["Name"] = i.NameTag

	return output.AddAWSTags(tags, i, "instance")
}
