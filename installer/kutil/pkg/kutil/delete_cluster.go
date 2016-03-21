package kutil

import (
	"fmt"
	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/fi"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"encoding/base64"
	"strings"
)

type DeleteCluster struct {
	ClusterID string
	Zone      string
	Cloud     fi.Cloud
}

func (c*DeleteCluster)  ListResources() ([]DeletableResource, error) {
	cloud := c.Cloud.(*fi.AWSCloud)

	var resources []DeletableResource

	filters := cloud.BuildFilters(nil)
	tags := cloud.BuildTags(nil)

	{
		glog.V(2).Infof("Listing all Autoscaling groups matching cluster tags")
		var asgNames []*string
		{
			var asFilters []*autoscaling.Filter
			for _, f := range filters {
				asFilters = append(asFilters, &autoscaling.Filter{
					Name: aws.String("value"),
					Values: f.Values,
				})
			}
			request := &autoscaling.DescribeTagsInput{
				Filters: asFilters,
			}
			response, err := cloud.Autoscaling.DescribeTags(request)
			if err != nil {
				return nil, fmt.Errorf("error listing autoscaling cluster tags: %v", err)
			}

			for _, t := range response.Tags {
				switch (*t.ResourceType) {
				case "auto-scaling-group":
					asgNames = append(asgNames, t.ResourceId)
				default:
					glog.Warningf("Unknown resource type: %v", *t.ResourceType)

				}
			}
		}

		if len(asgNames) != 0 {
			request := &autoscaling.DescribeAutoScalingGroupsInput{
				AutoScalingGroupNames: asgNames,
			}
			response, err := cloud.Autoscaling.DescribeAutoScalingGroups(request)
			if err != nil {
				return nil, fmt.Errorf("error listing autoscaling groups: %v", err)
			}

			for _, t := range response.AutoScalingGroups {
				if !matchesAsgTags(tags, t.Tags) {
					continue
				}
				resources = append(resources, &DeletableASG{Name: *t.AutoScalingGroupName })
			}
		}
	}

	{
		glog.V(2).Infof("Listing all Autoscaling LaunchConfigurations")

		request := &autoscaling.DescribeLaunchConfigurationsInput{
		}
		response, err := cloud.Autoscaling.DescribeLaunchConfigurations(request)
		if err != nil {
			return nil, fmt.Errorf("error listing autoscaling LaunchConfigurations: %v", err)
		}

		for _, t := range response.LaunchConfigurations {
			if t.UserData == nil {
				continue
			}

			userData, err := base64.StdEncoding.DecodeString(*t.UserData)
			if err != nil {
				glog.Infof("Ignoring autoscaling LaunchConfiguration with invalid UserData: %v", *t.LaunchConfigurationName)
				continue
			}

			if strings.Contains(string(userData), "\nINSTANCE_PREFIX: " + c.ClusterID + "\n") {
				resources = append(resources, &DeletableAutoscalingLaunchConfiguration{Name: *t.LaunchConfigurationName })
			}
		}
	}

	{

		glog.V(2).Infof("Listing all EC2 tags matching cluster tags")
		request := &ec2.DescribeTagsInput{
			Filters: filters,
		}
		response, err := cloud.EC2.DescribeTags(request)
		if err != nil {
			return nil, fmt.Errorf("error listing cluster tags: %v", err)
		}

		for _, t := range response.Tags {
			var resource DeletableResource
			switch (*t.ResourceType) {
			case "instance":
				resource = &DeletableInstance{ID: *t.ResourceId}
			case "volume":
				resource = &DeletableVolume{ID: *t.ResourceId}
			}

			if resource == nil {
				glog.Warningf("Unknown resource type: %v", *t.ResourceType)
				continue
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

func matchesAsgTags(tags map[string]string, actual []*autoscaling.TagDescription) bool {
	for k, v := range tags {
		found := false
		for _, a := range actual {
			if aws.StringValue(a.Key) == k {
				if aws.StringValue(a.Value) == v {
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

type DeletableResource interface {
	Delete(cloud fi.Cloud) error
}

type DeletableInstance struct {
	ID string
}

func (r*DeletableInstance) Delete(cloud fi.Cloud) error {
	c := cloud.(*fi.AWSCloud)

	glog.V(2).Infof("Deleting EC2 instance %q", r.ID)
	request := &ec2.TerminateInstancesInput{
		InstanceIds: []*string{&r.ID },
	}
	_, err := c.EC2.TerminateInstances(request)
	if err != nil {
		return fmt.Errorf("error deleting instance %q: %v", r.ID, err)
	}
	return nil
}
func (r*DeletableInstance) String() string {
	return "instance:" + r.ID
}

type DeletableVolume struct {
	ID string
}

func (r*DeletableVolume) Delete(cloud fi.Cloud) error {
	c := cloud.(*fi.AWSCloud)

	glog.V(2).Infof("Deleting EC2 volume %q", r.ID)
	request := &ec2.DeleteVolumeInput{
		VolumeId: &r.ID,
	}
	_, err := c.EC2.DeleteVolume(request)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == "InvalidVolume.NotFound" {
				return nil
			}
		}
		return fmt.Errorf("error deleting volume %q: %v", r.ID, err)
	}
	return nil
}
func (r*DeletableVolume) String() string {
	return "volume:" + r.ID
}

type DeletableASG struct {
	Name string
}

func (r*DeletableASG) Delete(cloud fi.Cloud) error {
	c := cloud.(*fi.AWSCloud)

	glog.V(2).Infof("Deleting autoscaling group %q", r.Name)
	request := &autoscaling.DeleteAutoScalingGroupInput{
		AutoScalingGroupName: &r.Name,
		ForceDelete: aws.Bool(true),
	}
	_, err := c.Autoscaling.DeleteAutoScalingGroup(request)
	if err != nil {
		return fmt.Errorf("error deleting autoscaling group %q: %v", r.Name, err)
	}
	return nil
}
func (r*DeletableASG) String() string {
	return "autoscaling-group:" + r.Name
}

type DeletableAutoscalingLaunchConfiguration struct {
	Name string
}

func (r*DeletableAutoscalingLaunchConfiguration) Delete(cloud fi.Cloud) error {
	c := cloud.(*fi.AWSCloud)

	glog.V(2).Infof("Deleting autoscaling LaunchConfiguration %q", r.Name)
	request := &autoscaling.DeleteLaunchConfigurationInput{
		LaunchConfigurationName: &r.Name,
	}
	_, err := c.Autoscaling.DeleteLaunchConfiguration(request)
	if err != nil {
		return fmt.Errorf("error deleting autoscaling LaunchConfiguration %q: %v", r.Name, err)
	}
	return nil
}

func (r*DeletableAutoscalingLaunchConfiguration) String() string {
	return "autoscaling-launchconfiguration:" + r.Name
}

