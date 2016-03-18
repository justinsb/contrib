package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/fi"
)

type AutoscalingLaunchConfiguration struct {
	InstanceCommonConfig
	Name *string
}

func (l *AutoscalingLaunchConfiguration) Prefix() string {
	return "AutoscalingLaunchConfiguration"
}

func (l *AutoscalingLaunchConfiguration) GetID() *string {
	return l.Name
}

func (l *AutoscalingLaunchConfiguration) String() string {
	return fmt.Sprintf("LaunchConfiguration (name=%s)", l.Name)
}

type AutoscalingLaunchConfigurationRenderer interface {
	RenderAutoscalingLaunchConfiguration(actual, expected, changes *AutoscalingLaunchConfiguration) error
}

func (e *AutoscalingLaunchConfiguration) find(c *fi.RunContext) (*AutoscalingLaunchConfiguration, error) {
	cloud := c.Cloud().(*fi.AWSCloud)

	request := &autoscaling.DescribeLaunchConfigurationsInput{
		LaunchConfigurationNames: []*string{e.Name},
	}

	response, err := cloud.Autoscaling.DescribeLaunchConfigurations(request)
	if err != nil {
		return nil, fmt.Errorf("error listing AutoscalingLaunchConfigurations: %v", err)
	}

	if response == nil || len(response.LaunchConfigurations) == 0 {
		return nil, nil
	}

	if len(response.LaunchConfigurations) != 1 {
		glog.Fatalf("found multiple AutoscalingLaunchConfigurations with name: %q", *e.Name)
	}
	glog.V(2).Info("found existing AutoscalingLaunchConfiguration")
	i := response.LaunchConfigurations[0]
	actual := &AutoscalingLaunchConfiguration{}
	actual.Name = i.LaunchConfigurationName
	actual.ImageID = i.ImageId
	actual.InstanceType = i.InstanceType
	actual.SSHKey = &SSHKey{Name:i.KeyName}

	securityGroups := []*SecurityGroup{}
	for _, sgID := range i.SecurityGroups {
		securityGroups = append(securityGroups, &SecurityGroup{ID:sgID})
	}
	actual.SecurityGroups = securityGroups
	actual.AssociatePublicIP = i.AssociatePublicIpAddress

	actual.BlockDeviceMappings = []*BlockDeviceMapping{}
	for _, b := range i.BlockDeviceMappings {
		actual.BlockDeviceMappings = append(actual.BlockDeviceMappings, BlockDeviceMappingFromAutoscaling(b))
	}
	actual.UserData = NewStringResource(*i.UserData)
	actual.IAMInstanceProfile = &IAMInstanceProfile{ID: i.IamInstanceProfile }
	actual.AssociatePublicIP = i.AssociatePublicIpAddress

	return actual, nil
}

func (e *AutoscalingLaunchConfiguration) Run(c *fi.RunContext) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &AutoscalingLaunchConfiguration{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	err = e.checkChanges(a, e, changes)
	if err != nil {
		return err
	}

	target := c.Target.(AutoscalingLaunchConfigurationRenderer)
	return target.RenderAutoscalingLaunchConfiguration(a, e, changes)
}

func (s *AutoscalingLaunchConfiguration) checkChanges(a, e, changes *AutoscalingLaunchConfiguration) error {
	if a != nil {
		if e.Name == nil {
			return MissingValueError("Name is required when creating AutoscalingLaunchConfiguration")
		}
	}
	return nil
}

func (t *AWSAPITarget) RenderAutoscalingLaunchConfiguration(a, e, changes *AutoscalingLaunchConfiguration) error {
	if a == nil {
		glog.V(2).Infof("Creating AutoscalingLaunchConfiguration with Name:%q", *e.Name)

		request := &autoscaling.CreateLaunchConfigurationInput{}
		request.LaunchConfigurationName = e.Name
		request.ImageId = e.ImageID
		request.InstanceType = e.InstanceType
		if e.SSHKey != nil {
			request.KeyName = e.SSHKey.Name
		}
		securityGroupIDs := []*string{}
		for _, sg := range e.SecurityGroups {
			securityGroupIDs = append(securityGroupIDs, sg.ID)
		}
		request.SecurityGroups = securityGroupIDs
		request.AssociatePublicIpAddress = e.AssociatePublicIP
		if e.BlockDeviceMappings != nil {
			request.BlockDeviceMappings = []*autoscaling.BlockDeviceMapping{}
			for _, b := range e.BlockDeviceMappings {
				request.BlockDeviceMappings = append(request.BlockDeviceMappings, b.ToAutoscaling())
			}
		}


		if e.UserData != nil {
			d, err := ResourceAsString(e.UserData)
			if err != nil {
				return fmt.Errorf("error rendering AutoScalingLaunchConfiguration UserData: %v", err)
			}
			request.UserData = &d
		}
		if e.IAMInstanceProfile != nil {
			request.IamInstanceProfile = e.IAMInstanceProfile.ID
		}

		_, err := t.cloud.Autoscaling.CreateLaunchConfiguration(request)
		if err != nil {
			return fmt.Errorf("error creating AutoscalingLaunchConfiguration: %v", err)
		}
	}

	return nil //return output.AddAWSTags(cloud.Tags(), v, "vpc")
}

func (t *BashTarget) RenderAutoscalingLaunchConfiguration(a, e, changes *AutoscalingLaunchConfiguration) error {
	t.CreateVar(e)
	if a == nil {
		glog.V(2).Infof("Creating AutoscalingLaunchConfiguration with Name:%q", *e.Name)

		args := []string{"create-launch-configuration"}
		args = append(args, "--launch-configuration-name", *e.Name)
		args = append(args, e.buildAutoscalingCreateArgs(t)...)

		t.AddAutoscalingCommand(args...)
	}

	return nil
}
