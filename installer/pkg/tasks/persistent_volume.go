package tasks

import (
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/fi"
)

type PersistentVolume struct {
	ID               *string
	AvailabilityZone *string
	VolumeType       *string
	Size             *int64
	Name             *string
}

type PersistentVolumeRenderer interface {
	RenderPersistentVolume(actual, expected, changes *PersistentVolume) error
}

func (s *PersistentVolume) Prefix() string {
	return "PersistentVolume"
}

func (s *PersistentVolume) GetID() *string {
	return s.ID
}

func (s *PersistentVolume) String() string {
	return BuildString(s)
}

func (e *PersistentVolume) find(c *fi.RunContext) (*PersistentVolume, error) {
	cloud := c.Cloud().(*fi.AWSCloud)

	filters := cloud.BuildFilters()
	filters = append(filters, fi.NewEC2Filter("tag:Name", *e.Name))
	request := &ec2.DescribeVolumesInput{
		Filters: filters,
	}

	response, err := cloud.EC2.DescribeVolumes(request)
	if err != nil {
		return nil, fmt.Errorf("error listing volumes: %v", err)
	}

	if response == nil || len(response.Volumes) == 0 {
		return nil, nil
	}

	if len(response.Volumes) != 1 {
		return nil, fmt.Errorf("found multiple Volumes with name: %s", *e.Name)
	}
	glog.V(2).Info("found existing volume")
	v := response.Volumes[0]
	actual := &PersistentVolume{}
	actual.ID = v.VolumeId
	actual.AvailabilityZone = v.AvailabilityZone
	actual.VolumeType = v.VolumeType
	actual.Size = v.Size
	actual.Name = e.Name
	return actual, nil
}

func (e *PersistentVolume) Run(c *fi.RunContext) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &PersistentVolume{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	err = e.checkChanges(a, e, changes)
	if err != nil {
		return err
	}

	target := c.Target.(PersistentVolumeRenderer)
	return target.RenderPersistentVolume(a, e, changes)
}

func (s *PersistentVolume) checkChanges(a, e, changes *PersistentVolume) error {
	if a == nil {
		if e.Name == nil {
			return MissingValueError("Name must be specified when creating a PersistentVolume")
		}
	}
	if a != nil {
		if changes.ID != nil {
			return InvalidChangeError("Cannot change PersistentVolume ID", changes.ID, e.ID)
		}
	}
	return nil
}

func (t *AWSAPITarget) RenderPersistentVolume(a, e, changes *PersistentVolume) error {
	if a == nil {
		glog.V(2).Infof("Creating PersistentVolume with Name:%q", *e.Name)

		request := &ec2.CreateVolumeInput{}
		request.Size = e.Size
		request.AvailabilityZone = e.AvailabilityZone
		request.VolumeType = e.VolumeType

		response, err := t.cloud.EC2.CreateVolume(request)
		if err != nil {
			return fmt.Errorf("error creating PersistentVolume: %v", err)
		}

		e.ID = response.VolumeId
	}

	tags := t.cloud.Tags()
	tags["Name"] = *e.Name

	return t.AddAWSTags(tags, e, "volume")
}

func (t *BashTarget) RenderPersistentVolume(a, e, changes *PersistentVolume) error {
	t.CreateVar(e)
	if a == nil {
		glog.V(2).Infof("Creating PersistentVolume with Name:%q", *e.Name)

		t.AddEC2Command("create-volume",
			"--availability-zone", *e.AvailabilityZone,
			"--volume-type", *e.VolumeType,
			"--size", strconv.FormatInt(*e.Size, 10),
			"--query", "VolumeId").AssignTo(e)
	} else {
		t.AddAssignment(e, StringValue(a.ID))
	}

	tags := t.cloud.Tags()
	tags["Name"] = *e.Name

	return t.AddAWSTags(tags, e, "volume")
}
