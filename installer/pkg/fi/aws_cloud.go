package fi

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"fmt"
	"github.com/golang/glog"
	fi_s3 "k8s.io/contrib/installer/pkg/fi/aws/s3"
)

type AWSCloud struct {
	Cloud

	EC2         *ec2.EC2
	S3          *fi_s3.S3Helper
	IAM         *iam.IAM
	Autoscaling *autoscaling.AutoScaling

	Region      string

	tags        map[string]string
}

var _ Cloud = &AWSCloud{}

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

func NewAWSCloud(region string, tags map[string]string) *AWSCloud {
	c := &AWSCloud{Region: region}

	config := aws.NewConfig().WithRegion(region)
	c.EC2 = ec2.New(session.New(), config)
	c.S3 = fi_s3.NewS3Helper(config)
	c.IAM = iam.New(session.New(), config)
	c.Autoscaling = autoscaling.New(session.New(), config)

	c.tags = tags
	return c
}

func (c*AWSCloud) GetS3(region string) *s3.S3 {
	return c.S3.GetS3(region)
}

func NewEC2Filter(name string, values ...string) *ec2.Filter {
	awsValues := []*string{}
	for _, value := range values {
		awsValues = append(awsValues, aws.String(value))
	}
	filter := &ec2.Filter{
		Name:   aws.String(name),
		Values: awsValues,
	}
	return filter
}

func (c *AWSCloud) Tags() map[string]string {
	// Defensive copy
	tags := make(map[string]string)
	for k, v := range c.tags {
		tags[k] = v
	}
	return tags
}

func (c *AWSCloud) GetTags(resourceId string, resourceType string) (map[string]string, error) {
	tags := map[string]string{}

	request := &ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			NewEC2Filter("resource-id", resourceId),
			NewEC2Filter("resource-type", resourceType),
		},
	}

	response, err := c.EC2.DescribeTags(request)
	if err != nil {
		return nil, fmt.Errorf("error listing tags on %v:%v: %v", resourceType, resourceId, err)
	}

	for _, tag := range response.Tags {
		if tag == nil {
			glog.Warning("unexpected nil tag")
			continue
		}
		tags[aws.StringValue(tag.Key)] = aws.StringValue(tag.Value)
	}

	return tags, nil
}

func (c *AWSCloud) CreateTags(resourceId string, resourceType string, tags map[string]string) (error) {
	if len(tags) == 0 {
		return nil
	}

	ec2Tags := []*ec2.Tag{}
	for k, v := range tags {
		ec2Tags = append(ec2Tags, &ec2.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	request := &ec2.CreateTagsInput{
		Tags: ec2Tags,
		Resources: []*string{&resourceId},
	}

	_, err := c.EC2.CreateTags(request)
	if err != nil {
		return fmt.Errorf("error creating tags on %v:%v: %v", resourceType, resourceId, err)
	}

	return nil
}

func (c *AWSCloud) BuildFilters() []*ec2.Filter {
	filters := []*ec2.Filter{}
	for name, value := range c.tags {
		filter := NewEC2Filter("tag:" + name, value)
		filters = append(filters, filter)
	}
	return filters
}

func (c *AWSCloud) EnvVars() map[string]string {
	env := map[string]string{}
	env["AWS_DEFAULT_REGION"] = c.Region
	env["AWS_DEFAULT_OUTPUT"] = "text"
	return env
}
