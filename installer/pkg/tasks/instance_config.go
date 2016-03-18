package tasks

import (
	"encoding/json"
	"strings"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"github.com/aws/aws-sdk-go/service/autoscaling"
)

type BlockDeviceMapping struct {
	DeviceName  *string
	VirtualName *string
}

func BlockDeviceMappingFromEC2(i *ec2.BlockDeviceMapping) *BlockDeviceMapping {
	o := &BlockDeviceMapping{}
	o.DeviceName = i.DeviceName
	o.VirtualName = i.VirtualName
	return o
}

func (i*BlockDeviceMapping) ToEC2() *ec2.BlockDeviceMapping {
	o := &ec2.BlockDeviceMapping{}
	o.DeviceName = i.DeviceName
	o.VirtualName = i.VirtualName
	return o
}

func BlockDeviceMappingFromAutoscaling(i *autoscaling.BlockDeviceMapping) *BlockDeviceMapping {
	o := &BlockDeviceMapping{}
	o.DeviceName = i.DeviceName
	o.VirtualName = i.VirtualName
	return o
}

func (i*BlockDeviceMapping) ToAutoscaling() *autoscaling.BlockDeviceMapping {
	o := &autoscaling.BlockDeviceMapping{}
	o.DeviceName = i.DeviceName
	o.VirtualName = i.VirtualName
	return o
}

// Config common to Instance and ASG LaunchConfiguration
type InstanceCommonConfig struct {
	ImageID             *string
	InstanceType        *string
	SSHKey              *SSHKey
	SecurityGroups      []*SecurityGroup
	AssociatePublicIP   *bool
	BlockDeviceMappings []*BlockDeviceMapping
	UserData            Resource
	IAMInstanceProfile  *IAMInstanceProfile
}

func (i *InstanceCommonConfig) buildCommonCreateArgs(output *BashTarget) []string {
	args := []string{}
	args = append(args, "--image-id", *i.ImageID)
	args = append(args, "--instance-type", *i.InstanceType)
	if i.SSHKey != nil {
		args = append(args, "--key-name", *i.SSHKey.Name)
	}
	if i.AssociatePublicIP != nil {
		if *i.AssociatePublicIP {
			args = append(args, "--associate-public-ip-address")
		} else {
			args = append(args, "--no-associate-public-ip-address")
		}
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
		args = append(args, "--user-data", "file://" + tempFile)
	}

	return args
}

func (i *InstanceCommonConfig) buildEC2CreateArgs(output *BashTarget) []string {
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
		args = append(args, "--iam-instance-profile", "Name=" + *i.IAMInstanceProfile.Name)
	}
	return args
}

func (i *InstanceCommonConfig) buildAutoscalingCreateArgs(output *BashTarget) []string {
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
		args = append(args, "--iam-instance-profile", *i.IAMInstanceProfile.Name)
	}
	return args
}