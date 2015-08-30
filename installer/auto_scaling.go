package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

type AutoScalingLaunchConfiguration struct {
	Name                string
	ImageID             string
	InstanceType        string
	SSHKey              *SSHKey
	SecurityGroups      []*SecurityGroup
	AssociatePublicIP   bool
	BlockDeviceMappings []ec2.BlockDeviceMapping
	UserData            Resource
	IAMInstanceProfile  *IAMInstanceProfile
}

func (l *AutoScalingLaunchConfiguration) Prefix() string {
	return "LaunchConfig"
}

func (l *AutoScalingLaunchConfiguration) String() string {
	return fmt.Sprintf("LaunchConfiguration (name=%s)", l.Name)
}

func (l *AutoScalingLaunchConfiguration) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	var existing *autoscaling.LaunchConfiguration

	name := l.Name

	request := &autoscaling.DescribeLaunchConfigurationsInput{
		LaunchConfigurationNames: []*string{&name},
	}

	response, err := cloud.autoscaling.DescribeLaunchConfigurations(request)
	if err != nil {
		return fmt.Errorf("error listing launch configurations: %v", err)
	}

	if response != nil {
		if len(response.LaunchConfigurations) != 0 {
			if len(response.LaunchConfigurations) != 1 {
				glog.Fatalf("found multiple launch configurations with name: %s", name)
			}
			glog.V(2).Info("found existing launch configuration")
			existing = response.LaunchConfigurations[0]
		}
	}

	// TODO: Validate existing

	output.CreateVar(i)
	if existing == nil {
		glog.V(2).Info("launch configuration not found; will create: ", i)
		args := []string{"create-launch-configuration"}
		// TODO: This is copy-and-pasted from instance.  Create InstanceConfig for shared config?
		args = append(args, "--launch-configuration-name", name)
		args = append(args, "--image-id", i.ImageID)
		args = append(args, "--instance-type", i.InstanceType)
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
			j, err := json.Marshal(i.BlockDeviceMappings)
			if err != nil {
				return fmt.Errorf("error converting BlockDeviceMappings to JSON: %v", err)
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
				return err
			}
			args = append(args, "--user-data", "file://"+tempFile)
		}
		if i.IAMInstanceProfile != nil {
			args = append(args, "--iam-instance-profile", i.IAMInstanceProfile.Name)
		}
		output.AddEC2Command(args...)
	}

	return nil
}
