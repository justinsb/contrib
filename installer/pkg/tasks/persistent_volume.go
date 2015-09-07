package tasks

import (
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

type PersistentVolume struct {
	AZ         string
	VolumeType string
	Size       int
	NameTag    string
}

func (v *PersistentVolume) Prefix() string {
	return "Volume"
}

func (v *PersistentVolume) String() string {
	return fmt.Sprintf("Peristent Volume (NameTag=%s)", v.NameTag)
}

func (v *PersistentVolume) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	var existing *ec2.Volume

	filters := cloud.BuildFilters()
	filters = append(filters, newEc2Filter("tag:Name", v.NameTag))
	request := &ec2.DescribeVolumesInput{
		Filters: filters,
	}

	response, err := cloud.ec2.DescribeVolumes(request)
	if err != nil {
		return fmt.Errorf("error listing volumes: %v", err)
	}

	if response != nil && len(response.Volumes) != 0 {
		if len(response.Volumes) != 1 {
			glog.Fatalf("found multiple volumes with name: %s", v.NameTag)
		}
		glog.V(2).Info("found existing volume")
		existing = response.Volumes[0]
	}

	output.CreateVar(v)
	if existing == nil {
		glog.V(2).Info("volume not found; will create: ", v)
		output.AddEC2Command("create-volume",
			"--availability-zone", v.AZ,
			"--volume-type", v.VolumeType,
			"--size", strconv.Itoa(v.Size),
			"--query", "VolumeId").AssignTo(v)
	} else {
		output.AddAssignment(v, aws.StringValue(existing.VolumeId))
	}

	tags := cloud.Tags()
	tags["Name"] = v.NameTag

	return output.AddAWSTags(tags, v, "volume")
}
