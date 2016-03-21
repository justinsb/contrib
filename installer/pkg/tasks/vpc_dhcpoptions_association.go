package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/ec2"
	"k8s.io/contrib/installer/pkg/fi"
)

type VPCDHCPOptionsAssociationRenderer interface {
	RenderVPCDHCPOptionsAssociation(actual, expected, changes *VPCDHCPOptionsAssociation) error
}

type VPCDHCPOptionsAssociation struct {
	fi.SimpleUnit

	VPC         *VPC
	DHCPOptions *DHCPOptions
}

func (s *VPCDHCPOptionsAssociation) Key() string {
	return s.VPC.Key() + "-" + s.DHCPOptions.Key()
}

func (e *VPCDHCPOptionsAssociation) find(c *fi.RunContext) (*VPCDHCPOptionsAssociation, error) {
	cloud := c.Cloud().(*fi.AWSCloud)

	vpcID := e.VPC.ID
	dhcpOptionsID := e.DHCPOptions.ID

	if vpcID == nil || dhcpOptionsID == nil {
		return nil, nil
	}

	request := &ec2.DescribeVpcsInput{
		VpcIds: []*string{vpcID },
	}
	response, err := cloud.EC2.DescribeVpcs(request)
	if err != nil {
		return nil, err
	}

	if response == nil || len(response.Vpcs) == 0 {
		return nil, nil
	}
	if len(response.Vpcs) != 1 {
		return nil, fmt.Errorf("found multiple VPCs with id %q", vpcID)
	}

	vpc := response.Vpcs[0]
	actual := &VPCDHCPOptionsAssociation{}
	actual.VPC = &VPC{ID: vpc.VpcId }
	actual.DHCPOptions = &DHCPOptions{ID: vpc.DhcpOptionsId }
	return actual, nil
}

func (e *VPCDHCPOptionsAssociation) Run(c *fi.RunContext) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &VPCDHCPOptionsAssociation{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	err = e.checkChanges(a, e, changes)
	if err != nil {
		return err
	}

	target := c.Target.(VPCDHCPOptionsAssociationRenderer)
	return target.RenderVPCDHCPOptionsAssociation(a, e, changes)
}

func (s *VPCDHCPOptionsAssociation) checkChanges(a, e, changes *VPCDHCPOptionsAssociation) error {
	if a == nil {
		if e.DHCPOptions == nil || e.VPC == nil {
			return MissingValueError("Must specify VPC and DHCPOptions for VPCDHCPOptionsAssociation creation")
		}
	}
	return nil
}

func (t *AWSAPITarget) RenderVPCDHCPOptionsAssociation(a, e, changes *VPCDHCPOptionsAssociation) error {
	if a == nil {
		request := &ec2.AssociateDhcpOptionsInput{}
		request.VpcId = e.VPC.ID
		request.DhcpOptionsId = e.DHCPOptions.ID

		_, err := t.cloud.EC2.AssociateDhcpOptions(request)
		if err != nil {
			return fmt.Errorf("error creating VPCDHCPOptionsAssociation: %v", err)
		}
	}

	return nil // no tags
}

func (t *BashTarget) RenderVPCDHCPOptionsAssociation(a, e, changes *VPCDHCPOptionsAssociation) error {
	//t.CreateVar(e)
	if a == nil {
		vpcID := t.ReadVar(e.VPC)
		dhcpOptionsID := t.ReadVar(e.DHCPOptions)

		t.AddEC2Command("associate-dhcp-options", "--dhcp-options-id", dhcpOptionsID, "--vpc-id", vpcID)
	} else {
		//t.AddAssignment(e, StringValue(a.ID))
	}

	return nil // no tags
}
