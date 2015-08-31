package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

// Config common to Instance and ASG LaunchConfiguration
type InstanceConfig struct {
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

func (i *InstanceConfig) buildCommonCreateArgs(output *BashTarget) []string {
	args := []string{}
	args = append(args, "--image-id", i.ImageID)
	args = append(args, "--instance-type", i.InstanceType)
	if i.Subnet != nil {
		args = append(args, "--subnet-id", output.ReadVar(i.Subnet))
	}
	if i.PrivateIPAddress != "" {
		args = append(args, "--private-ip-address", i.PrivateIPAddress)
	}
	if i.SSHKey != nil {
		args = append(args, "--key-name", i.SSHKey.Name)
	}
	if i.AssociatePublicIP {
		args = append(args, "--associate-public-ip-address")
	} else {
		args = append(args, "--no-associate-public-ip-address")
	}
	if i.BlockDeviceMappings != nil {
		j, err := json.Marshal(i.BlockDeviceMappings)
		if err != nil {
			glog.Fatalf("error converting BlockDeviceMappings to JSON: %v", err)
		}

		bdm := string(j)
		// Hack to remove null values
		bdm = strings.Replace(bdm, "\"Ebs\":null,", "", -1)
		bdm = strings.Replace(bdm, "\"NoDevice\":null,", "", -1)
		bdm = strings.Replace(bdm, "\"VirtualName\":null,", "", -1)

		args = append(args, "--block-device-mappings", bashQuoteString(bdm))
	}
	if i.UserData != nil {
		tempFile, err := output.AddResource(i.UserData)
		if err != nil {
			glog.Fatalf("error adding resource: %v", err)
		}
		args = append(args, "--user-data", "file://"+tempFile)
	}

	return args
}

func (i *InstanceConfig) buildEC2CreateArgs(output *BashTarget) []string {
	args := i.buildCommonCreateArgs(output)
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
	if i.IAMInstanceProfile != nil {
		args = append(args, "--iam-instance-profile", "Name="+i.IAMInstanceProfile.Name)
	}
	return args
}

func (i *InstanceConfig) buildAutoscalingCreateArgs(output *BashTarget) []string {
	args := i.buildCommonCreateArgs(output)
	if i.SecurityGroups != nil {
		ids := ""
		for _, sg := range i.SecurityGroups {
			if ids != "" {
				ids = ids + ","
			}
			ids = ids + output.ReadVar(sg)
		}
		args = append(args, "--security-groups", ids)
	}
	if i.IAMInstanceProfile != nil {
		args = append(args, "--iam-instance-profile", i.IAMInstanceProfile.Name)
	}
	return args
}

type Instance struct {
	InstanceConfig

	NameTag string
	Tags    map[string]string
}

func (i *Instance) Prefix() string {
	return "Instance"
}

func (i *Instance) String() string {
	return fmt.Sprintf("Instance (name=%s)", i.NameTag)
}

func (i *Instance) findExisting(cloud *AWSCloud) (*ec2.Instance, error) {
	var existing *ec2.Instance

	filters := cloud.BuildFilters()
	filters = append(filters, newEc2Filter("tag:Name", i.NameTag))
	request := &ec2.DescribeInstancesInput{
		Filters: filters,
	}

	response, err := cloud.ec2.DescribeInstances(request)
	if err != nil {
		return nil, fmt.Errorf("error listing instances: %v", err)
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

	return existing, nil
}

func (i *Instance) Destroy(cloud *AWSCloud, output *BashTarget) error {
	existing, err := i.findExisting(cloud)
	if err != nil {
		return err
	}

	if existing != nil {
		glog.V(2).Info("Found instance; will delete: ", i)
		args := []string{"terminate-instances"}
		args = append(args, "--instance-ids", aws.StringValue(existing.InstanceId))

		output.AddEC2Command(args...).AssignTo(i)
	}

	return nil
}

func (i *Instance) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	existing, err := i.findExisting(cloud)
	if err != nil {
		return err
	}
	// TODO: Validate existing

	output.CreateVar(i)
	if existing == nil {
		glog.V(2).Info("instance not found; will create: ", i)
		args := []string{"run-instances"}
		args = append(args, i.buildEC2CreateArgs(output)...)

		args = append(args, "--query", "Instances[0].InstanceId")

		output.AddEC2Command(args...).AssignTo(i)
	} else {
		output.AddAssignment(i, aws.StringValue(existing.InstanceId))
	}

	tags := cloud.Tags()
	tags["Name"] = i.NameTag

	if i.Tags != nil {
		for k, v := range i.Tags {
			tags[k] = v
		}
	}

	return output.AddAWSTags(tags, i, "instance")
}
