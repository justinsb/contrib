package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

type SecurityGroupIngressRenderer interface {
	RenderSecurityGroupIngress(actual, expected, changes *SecurityGroupIngress) error
}

type SecurityGroupIngress struct {
	SecurityGroup *SecurityGroup
	CIDR          *string
	Protocol      *string
	FromPort      *int64
	ToPort        *int64
	SourceGroup   *SecurityGroup
}

func (e *SecurityGroupIngress) find(c *Context) (*SecurityGroupIngress, error) {
	cloud := c.Cloud

	var sourceGroupID *string
	canMatch := false
	if e.CIDR != nil {
		canMatch = true
	}
	if e.SourceGroup != nil {
		sourceGroupID = e.SourceGroup.ID
		if sourceGroupID != nil {
			canMatch = true
		}
	}

	var foundRule *ec2.IpPermission

	var sgID *string
	if e.SecurityGroup != nil {
		sgID = e.SecurityGroup.ID
	}

	if canMatch && sgID != nil {
		request := &ec2.DescribeSecurityGroupsInput{
			Filters: []*ec2.Filter{
				newEc2Filter("group-id", *sgID),
			},
		}

		response, err := cloud.EC2.DescribeSecurityGroups(request)
		if err != nil {
			return nil, fmt.Errorf("error listing security groups: %v", err)
		}

		var existing *ec2.SecurityGroup
		if response != nil && len(response.SecurityGroups) != 0 {
			if len(response.SecurityGroups) != 1 {
				glog.Fatalf("found multiple security groups for id=%s", *sgID)
			}
			glog.V(2).Info("found existing security group")
			existing = response.SecurityGroups[0]
		}

		if existing != nil {
			matchProtocol := "-1" // Wildcard
			if e.Protocol != nil {
				matchProtocol = *e.Protocol
			}

			for _, rule := range existing.IpPermissions {
				if aws.Int64Value(rule.FromPort) != aws.Int64Value(e.FromPort) {
					continue
				}
				if aws.Int64Value(rule.ToPort) != aws.Int64Value(e.ToPort) {
					continue
				}
				if aws.StringValue(rule.IpProtocol) != matchProtocol {
					continue
				}
				match := false
				if e.CIDR != nil {
					for _, ipRange := range rule.IpRanges {
						if aws.StringValue(ipRange.CidrIp) == *e.CIDR {
							match = true
							break
						}
					}
				} else if sourceGroupID != nil {
					for _, spec := range rule.UserIdGroupPairs {
						if aws.StringValue(spec.GroupId) == *sourceGroupID {
							match = true
							break
						}
					}
				} else {
					glog.Fatalf("Expected CIDR or FromSecurityGroupIngress in %v", e)
				}
				if !match {
					continue
				}
				foundRule = rule
				break
			}
		}
	}

	if foundRule != nil {
		actual := &SecurityGroupIngress{}
		actual.FromPort = foundRule.FromPort
		actual.ToPort = foundRule.ToPort
		actual.Protocol = foundRule.IpProtocol
		if aws.StringValue(actual.Protocol) == "-1" {
			actual.Protocol = nil
		}
		return actual, nil
	}

	return nil, nil
}

func (e *SecurityGroupIngress) Run(c *Context) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &SecurityGroupIngress{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	err = e.checkChanges(a, e, changes)
	if err != nil {
		return err
	}

	target := c.Target.(SecurityGroupIngressRenderer)
	return target.RenderSecurityGroupIngress(a, e, changes)
}

func (s *SecurityGroupIngress) checkChanges(a, e, changes *SecurityGroupIngress) error {
	if a != nil {
		if changes.SecurityGroup == nil {
			return MissingValueError("Must specify SecurityGroup when creating SecurityGroupIngress")
		}
	}
	return nil
}

func (t *AWSAPITarget) RenderSecurityGroupIngress(a, e, changes *SecurityGroupIngress) error {
	if a == nil {
		request := &ec2.AuthorizeSecurityGroupIngressInput{}
		request.GroupId = e.SecurityGroup.ID
		request.CidrIp = e.CIDR
		request.IpProtocol = e.Protocol
		request.FromPort = e.FromPort
		request.ToPort = e.ToPort
		if e.SourceGroup != nil {
			request.IpPermissions = []*ec2.IpPermission{
				{
					UserIdGroupPairs: []*ec2.UserIdGroupPair{
						{
							GroupId: e.SourceGroup.ID,
						},
					},
				},
			}
		}
		_, err := t.cloud.EC2.AuthorizeSecurityGroupIngress(request)
		if err != nil {
			return fmt.Errorf("error creating SecurityGroupIngress: %v", err)
		}
	}

	return nil //return output.AddAWSTags(cloud.Tags(), v, "vpc")
}

func (t *BashTarget) RenderSecurityGroupIngress(a, e, changes *SecurityGroupIngress) error {
	if a == nil {
		glog.V(2).Infof("Creating SecurityGroupIngress")

		args := []string{"authorize-security-group-ingress"}
		args = append(args, "--group-id", t.ReadVar(e.SecurityGroup))

		if e.Protocol != nil {
			args = append(args, "--protocol", *e.Protocol)
		} else {
			args = append(args, "--protocol", "all")
		}
		fromPort := aws.Int64Value(e.FromPort)
		toPort := aws.Int64Value(e.ToPort)
		if fromPort != 0 || toPort != 0 {
			if fromPort == toPort {
				args = append(args, "--port", fmt.Sprintf("%d", fromPort))
			} else {
				args = append(args, "--port", fmt.Sprintf("%d-%d", fromPort, toPort))
			}
		}
		if e.CIDR != nil {
			args = append(args, "--cidr", *e.CIDR)
		}

		if e.SourceGroup != nil {
			args = append(args, "--source-group", t.ReadVar(e.SourceGroup))
		}

		t.AddEC2Command(args...)
	}

	return nil
}

func (s *SecurityGroupIngress) String() string {
	return fmt.Sprintf("SecurityGroupIngress (Port=%d-%d)", s.FromPort, s.ToPort)
}

func (s *SecurityGroupIngress) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	return nil
}
