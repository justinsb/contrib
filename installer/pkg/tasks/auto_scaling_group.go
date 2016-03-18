package tasks

import (
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/golang/glog"
	"github.com/aws/aws-sdk-go/aws"
	"strings"
)

type AutoscalingGroup struct {
	Name                *string
	LaunchConfiguration *AutoscalingLaunchConfiguration
	MinSize             *int64
	MaxSize             *int64
	Subnet              *Subnet
	Tags                map[string]string
}

type AutoscalingGroupRenderer interface {
	RenderAutoscalingGroup(actual, expected, changes *AutoscalingGroup) error
}

func (s *AutoscalingGroup) Prefix() string {
	return "AutoscalingGroup"
}

func (s *AutoscalingGroup) GetID() *string {
	return s.Name
}

func (e *AutoscalingGroup) find(c *Context) (*AutoscalingGroup, error) {
	cloud := c.Cloud

	request := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{e.Name},
	}

	response, err := cloud.Autoscaling.DescribeAutoScalingGroups(request)
	if err != nil {
		return nil, fmt.Errorf("error listing AutoscalingGroups: %v", err)
	}

	if response == nil || len(response.AutoScalingGroups) == 0 {
		return nil, nil
	}

	if len(response.AutoScalingGroups) != 1 {
		glog.Fatalf("found multiple AutoscalingGroups with name: %q", e.Name)
	}

	g := response.AutoScalingGroups[0]
	actual := &AutoscalingGroup{}
	actual.Name = g.AutoScalingGroupName
	actual.MinSize = g.MinSize
	actual.MaxSize = g.MaxSize

	if g.VPCZoneIdentifier != nil {
		subnets := strings.Split(*g.VPCZoneIdentifier, ",")
		if len(subnets) != 1 {
			panic("Multiple subnets not implemented in AutoScalingGroup")
		}
		for _, subnet := range subnets {
			actual.Subnet = &Subnet{ID: aws.String(subnet)}
		}
	}

	actual.LaunchConfiguration = &AutoscalingLaunchConfiguration{Name:g.LaunchConfigurationName}

	if len(g.Tags) != 0 {
		actual.Tags = make(map[string]string)
		for _, tag := range g.Tags {
			actual.Tags[*tag.Key] = *tag.Value
		}
	}

	return actual, nil
}

func (e *AutoscalingGroup) Run(c *Context) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &AutoscalingGroup{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	err = e.checkChanges(a, e, changes)
	if err != nil {
		return err
	}

	target := c.Target.(AutoscalingGroupRenderer)
	return target.RenderAutoscalingGroup(a, e, changes)
}

func (s *AutoscalingGroup) checkChanges(a, e, changes *AutoscalingGroup) error {
	if a != nil {
		if e.Name == nil {
			return MissingValueError("Name is required when creating AutoscalingGroup")
		}
	}
	return nil
}

func (t *AWSAPITarget) RenderAutoscalingGroup(a, e, changes *AutoscalingGroup) error {
	if a == nil {
		glog.V(2).Infof("Creating AutoscalingGroup with Name:%q", *e.Name)

		request := &autoscaling.CreateAutoScalingGroupInput{}
		request.AutoScalingGroupName = e.Name
		request.LaunchConfigurationName = e.LaunchConfiguration.Name
		request.MinSize = e.MinSize
		request.MaxSize = e.MaxSize
		request.VPCZoneIdentifier = e.Subnet.ID

		tags := []*autoscaling.Tag{}
		for k, v := range e.Tags {
			tags = append(tags, &autoscaling.Tag{Key:aws.String(k), Value: aws.String(v)})
		}
		request.Tags = tags

		_, err := t.cloud.Autoscaling.CreateAutoScalingGroup(request)
		if err != nil {
			return fmt.Errorf("error creating AutoscalingGroup: %v", err)
		}
	}

	return nil //return output.AddAWSTags(cloud.Tags(), v, "vpc")
}

func (t *BashTarget) RenderAutoscalingGroup(a, e, changes *AutoscalingGroup) error {
	if a == nil {
		glog.V(2).Infof("Creating AutoscalingGroup with Name:%q", *e.Name)

		args := []string{"create-auto-scaling-group"}
		args = append(args, "--auto-scaling-group-name", *e.Name)
		args = append(args, "--launch-configuration-name", *e.LaunchConfiguration.Name)
		args = append(args, "--min-size", strconv.FormatInt(*e.MinSize, 10))
		args = append(args, "--max-size", strconv.FormatInt(*e.MaxSize, 10))
		args = append(args, "--vpc-zone-identifier", t.ReadVar(e.Subnet))

		if e.Tags != nil && len(e.Tags) != 0 {
			args = append(args, "--tags")
			for k, v := range e.Tags {
				args = append(args, fmt.Sprintf("ResourceId=%s,ResourceType=auto-scaling-group,Key=%s,Value=%s", e.Name, k, v))
			}
		}

		t.AddAutoscalingCommand(args...)
	}

	return nil
}

/*
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
*/
