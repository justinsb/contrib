package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

type SecurityGroup struct {
	Name        string
	VPC         *VPC
	Description string
}

type SecurityGroupIngress struct {
	SecurityGroup *SecurityGroup
	CIDR          string
	Protocol      string
	FromPort      int64
	ToPort        int64
	SourceGroup   *SecurityGroup
}

func (s *SecurityGroup) AllowFrom(source *SecurityGroup) *SecurityGroupIngress {
	return &SecurityGroupIngress{SecurityGroup: s, SourceGroup: source}
}

func (s *SecurityGroup) AllowTCP(cidr string, fromPort int, toPort int) *SecurityGroupIngress {
	return &SecurityGroupIngress{
		SecurityGroup: s,
		CIDR:          cidr,
		Protocol:      "tcp",
		FromPort:      int64(fromPort),
		ToPort:        int64(toPort),
	}
}

func (s *SecurityGroup) Prefix() string {
	return "SecurityGroup"
}

func (s *SecurityGroup) String() string {
	return fmt.Sprintf("SecurityGroup (name=%s)", s.Name)
}

func (s *SecurityGroup) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	vpcId, _ := output.FindValue(s.VPC)

	sgId := ""
	if vpcId != "" {
		request := &ec2.DescribeSecurityGroupsInput{
			Filters: []*ec2.Filter{
				newEc2Filter("vpc-id", vpcId),
				newEc2Filter("group-name", s.Name)},
		}

		response, err := cloud.EC2.DescribeSecurityGroups(request)
		if err != nil {
			return fmt.Errorf("error listing security groups: %v", err)
		}

		var existing *ec2.SecurityGroup
		if response != nil && len(response.SecurityGroups) != 0 {
			if len(response.SecurityGroups) != 1 {
				glog.Fatalf("found multiple security groups for vpc=%s, name=%s", vpcId, s.Name)
			}
			glog.V(2).Info("found existing security group")
			existing = response.SecurityGroups[0]
			sgId = aws.StringValue(existing.GroupId)
		}
	}

	output.CreateVar(s)
	if sgId == "" {
		glog.V(2).Info("Security group not found; will create: ", s)
		output.AddEC2Command("create-security-group", "--group-name", s.Name,
			"--description", bashQuoteString(s.Description),
			"--vpc-id", output.ReadVar(s.VPC),
			"--query", "GroupId").AssignTo(s)
	} else {
		output.AddAssignment(s, sgId)
	}

	return output.AddAWSTags(cloud.Tags(), s, "security-group")
}

func (s *SecurityGroupIngress) String() string {
	return fmt.Sprintf("SecurityGroupIngress (Port=%d-%d)", s.FromPort, s.ToPort)
}

func (s *SecurityGroupIngress) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	sgId, _ := output.FindValue(s.SecurityGroup)

	sourceGroupID := ""
	canMatch := false
	if s.CIDR != "" {
		canMatch = true
	}
	if s.SourceGroup != nil {
		sourceGroupID, _ = output.FindValue(s.SourceGroup)
		if sourceGroupID != "" {
			canMatch = true
		}
	}

	var foundRule *ec2.IpPermission

	if canMatch && sgId != "" {
		request := &ec2.DescribeSecurityGroupsInput{
			Filters: []*ec2.Filter{
				newEc2Filter("group-id", sgId),
			},
		}

		response, err := cloud.EC2.DescribeSecurityGroups(request)
		if err != nil {
			return fmt.Errorf("error listing security groups: %v", err)
		}

		var existing *ec2.SecurityGroup
		if response != nil && len(response.SecurityGroups) != 0 {
			if len(response.SecurityGroups) != 1 {
				glog.Fatalf("found multiple security groups for id=%s", sgId)
			}
			glog.V(2).Info("found existing security group")
			existing = response.SecurityGroups[0]
		}

		if existing != nil {
			matchProtocol := s.Protocol
			if matchProtocol == "" {
				matchProtocol = "-1"
			}

			for _, rule := range existing.IpPermissions {
				if aws.Int64Value(rule.FromPort) != s.FromPort {
					continue
				}
				if aws.Int64Value(rule.ToPort) != s.ToPort {
					continue
				}
				if aws.StringValue(rule.IpProtocol) != matchProtocol {
					continue
				}
				match := false
				if s.CIDR != "" {
					for _, ipRange := range rule.IpRanges {
						if aws.StringValue(ipRange.CidrIp) == s.CIDR {
							match = true
							break
						}
					}
				} else if s.SourceGroup != nil {
					for _, spec := range rule.UserIdGroupPairs {
						if aws.StringValue(spec.GroupId) == sourceGroupID {
							match = true
							break
						}
					}
				} else {
					glog.Fatal("Expected CIDR or FromSecurityGroup in ", s)
				}
				if !match {
					continue
				}
				foundRule = rule
				break
			}
		}
	}

	if foundRule == nil {
		glog.V(2).Info("Security group ingress rule not found; will create: ", s)
		args := []string{"authorize-security-group-ingress"}
		args = append(args, "--group-id", output.ReadVar(s.SecurityGroup))

		if s.Protocol != "" {
			args = append(args, "--protocol", s.Protocol)
		} else {
			args = append(args, "--protocol", "all")
		}
		if s.FromPort != 0 || s.ToPort != 0 {
			if s.FromPort == s.ToPort {
				args = append(args, "--port", fmt.Sprintf("%d", s.FromPort))
			} else {
				args = append(args, "--port", fmt.Sprintf("%d-%d", s.FromPort, s.ToPort))
			}
		}
		if s.CIDR != "" {
			args = append(args, "--cidr", s.CIDR)
		}

		if s.SourceGroup != nil {
			args = append(args, "--source-group", output.ReadVar(s.SourceGroup))
		}

		output.AddEC2Command(args...)
	}

	return nil
}
