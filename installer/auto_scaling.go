package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/golang/glog"
)

type AutoscalingLaunchConfiguration struct {
	InstanceConfig
	Name string
}

func (l *AutoscalingLaunchConfiguration) Prefix() string {
	return "LaunchConfig"
}

func (l *AutoscalingLaunchConfiguration) String() string {
	return fmt.Sprintf("LaunchConfiguration (name=%s)", l.Name)
}

func (l *AutoscalingLaunchConfiguration) RenderBash(cloud *AWSCloud, output *BashTarget) error {
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

	if existing == nil {
		glog.V(2).Info("launch configuration not found; will create: ", l)
		args := []string{"create-launch-configuration"}
		args = append(args, "--launch-configuration-name", name)
		args = append(args, l.buildAutoscalingCreateArgs(output)...)

		output.AddAutoscalingCommand(args...)
	}

	return nil
}
