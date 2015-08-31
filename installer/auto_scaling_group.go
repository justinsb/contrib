package main

import (
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/golang/glog"
)

type AutoscalingGroup struct {
	Name                string
	LaunchConfiguration *AutoscalingLaunchConfiguration
	MinSize             int
	MaxSize             int
	Subnet              *Subnet
	Tags                map[string]string
}

func (g *AutoscalingGroup) String() string {
	return fmt.Sprintf("AutoscalingGroup (name=%s)", g.Name)
}

func (g *AutoscalingGroup) findExisting(cloud *AWSCloud) (*autoscaling.Group, error) {
	var existing *autoscaling.Group

	name := g.Name

	request := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{&name},
	}

	response, err := cloud.autoscaling.DescribeAutoScalingGroups(request)
	if err != nil {
		return nil, fmt.Errorf("error listing autoscaling groups: %v", err)
	}

	if response != nil {
		if len(response.AutoScalingGroups) != 0 {
			if len(response.AutoScalingGroups) != 1 {
				glog.Fatalf("found multiple autoscaling groups with name: %s", name)
			}
			glog.V(2).Info("found existing autoscaling group")
			existing = response.AutoScalingGroups[0]
		}
	}

	return existing, nil
}

func (g *AutoscalingGroup) Destroy(cloud *AWSCloud, output *BashTarget) error {
	existing, err := g.findExisting(cloud)
	if err != nil {
		return err
	}

	if existing != nil {
		glog.V(2).Info("found autoscaling group; will delete: ", g)
		args := []string{"delete-auto-scaling-group"}
		args = append(args, "--auto-scaling-group-name", g.Name)
		args = append(args, "--force-delete")

		output.AddAutoscalingCommand(args...)
	}

	return nil
}

func (g *AutoscalingGroup) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	existing, err := g.findExisting(cloud)
	if err != nil {
		return err
	}

	// TODO: Validate existing

	if existing == nil {
		glog.V(2).Info("autoscaling group not found; will create: ", g)
		args := []string{"create-auto-scaling-group"}
		args = append(args, "--auto-scaling-group-name", g.Name)
		args = append(args, "--launch-configuration-name", g.LaunchConfiguration.Name)
		args = append(args, "--min-size", strconv.Itoa(g.MinSize))
		args = append(args, "--max-size", strconv.Itoa(g.MaxSize))
		args = append(args, "--vpc-zone-identifier", output.ReadVar(g.Subnet))

		tags := cloud.Tags()

		if g.Tags != nil && len(g.Tags) != 0 {
			for k, v := range g.Tags {
				tags[k] = v
			}
		}

		if len(tags) != 0 {
			args = append(args, "--tags")
			for k, v := range tags {
				args = append(args, fmt.Sprintf("ResourceId=%s,ResourceType=auto-scaling-group,Key=%s,Value=%s", g.Name, k, v))
			}
		}

		output.AddAutoscalingCommand(args...)
	}

	return nil
}
