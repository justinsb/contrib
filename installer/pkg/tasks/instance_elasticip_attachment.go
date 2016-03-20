package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/fi"
)

type InstanceElasticIPAttachmentRenderer interface {
	RenderInstanceElasticIPAttachment(actual, expected, changes *InstanceElasticIPAttachment) error
}

type InstanceElasticIPAttachment struct {
	fi.SimpleUnit

	Instance  *Instance
	ElasticIP *ElasticIP
}

func (s *InstanceElasticIPAttachment) Key() string {
	return s.Instance.Key() + "-" + s.ElasticIP.Key()
}

func (e *InstanceElasticIPAttachment) find(c *fi.RunContext) (*InstanceElasticIPAttachment, error) {
	cloud := c.Cloud().(*fi.AWSCloud)

	instanceID := e.Instance.ID
	eipID := e.ElasticIP.ID

	if instanceID == nil || eipID == nil {
		return nil, nil
	}

	request := &ec2.DescribeAddressesInput{
		AllocationIds: []*string{eipID},
	}

	response, err := cloud.EC2.DescribeAddresses(request)
	if err != nil {
		return nil, fmt.Errorf("error listing ElasticIPs: %v", err)
	}
	if response == nil || len(response.Addresses) == 0 {
		return nil, nil
	}

	if len(response.Addresses) != 1 {
		glog.Fatalf("found multiple ElasticIPs for public IP")
	}

	a := response.Addresses[0]
	actual := &InstanceElasticIPAttachment{}
	if a.InstanceId != nil {
		actual.Instance = &Instance{ID: a.InstanceId }
	}
	actual.ElasticIP = &ElasticIP{ID: a.AllocationId }
	return actual, nil
}

func (e *InstanceElasticIPAttachment) Run(c *fi.RunContext) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &InstanceElasticIPAttachment{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	err = e.checkChanges(a, e, changes)
	if err != nil {
		return err
	}

	target := c.Target.(InstanceElasticIPAttachmentRenderer)
	return target.RenderInstanceElasticIPAttachment(a, e, changes)
}

func (s *InstanceElasticIPAttachment) checkChanges(a, e, changes *InstanceElasticIPAttachment) error {
	return nil
}

func (t *AWSAPITarget) RenderInstanceElasticIPAttachment(a, e, changes *InstanceElasticIPAttachment) error {
	if changes.Instance != nil {
		request := &ec2.AssociateAddressInput{}
		request.InstanceId = e.Instance.ID
		request.AllocationId = a.ElasticIP.ID

		_, err := t.cloud.EC2.AssociateAddress(request)
		if err != nil {
			return fmt.Errorf("error creating InstanceElasticIPAttachment: %v", err)
		}
	}

	return nil // no tags
}

func (t *BashTarget) RenderInstanceElasticIPAttachment(a, e, changes *InstanceElasticIPAttachment) error {
	//t.CreateVar(e)
	if a == nil {
		instanceID := t.ReadVar(e.Instance)
		allocationID := t.ReadVar(e.ElasticIP)

		t.AddBashCommand("wait-for-instance-state", instanceID, "running")
		t.AddEC2Command("associate-address", "--allocation-id", allocationID, "--instance-id", instanceID)
	} else {
		//t.AddAssignment(e, StringValue(a.ID))
	}

	return nil // no tags
}
