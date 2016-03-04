package fi

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
)

type ProviderID string

const ProviderAWS ProviderID = "aws"

type Cloud interface {
	ProviderID() ProviderID
}

/*
func (c *Cloud) init() error {
	// For EC2, check /sys/hypervisor/uuid, then check 169.254.169.254
	//	cat /sys/hypervisor/uuid
	//	ec21884e-23e4-dcf2-d27e-4495fedb2abd

	// curl http://169.254.169.254/
	glog.Warning("Cloud detection hard-coded")

	c.ProviderID = "aws"

	return nil
}
*/
/*
func (c *Cloud) IsAWS() bool {
	return c.ProviderID == "aws"
}

func (c *Cloud) IsGCE() bool {
	return c.ProviderID == "gce"
}

func (c *Cloud) IsVagrant() bool {
	return c.ProviderID == "vagrant"
}
*/
type AWSCloud struct {
	Cloud

	EC2         *ec2.EC2
	S3          *s3.S3
	IAM         *iam.IAM
	Autoscaling *autoscaling.AutoScaling
}

func (c *AWSCloud) IsAWS() bool {
	return true
}

func (c *AWSCloud) IsGCE() bool {
	return false
}

func (c *AWSCloud) IsVagrant() bool {
	return false
}

func (c *AWSCloud) ProviderID() ProviderID {
	return ProviderAWS
}

func NewAWSCloud() *AWSCloud {
	c := &AWSCloud{}
	config := aws.NewConfig() //.WithRegion(region)
	c.EC2 = ec2.New(session.New(), config)
	c.S3 = s3.New(session.New(), config)
	c.IAM = iam.New(session.New(), config)
	c.Autoscaling = autoscaling.New(session.New(), config)
	return c
}
