package tasks

import (
	"bytes"
	crypto_rand "crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/golang/glog"
)

func RandomToken(length int) string {
	// This is supposed to be the same algorithm as the old bash algorithm
	// KUBELET_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)
	// KUBE_PROXY_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)

	for {
		buffer := make([]byte, length*4)
		_, err := crypto_rand.Read(buffer)
		if err != nil {
			glog.Fatalf("error generating random token: %v", err)
		}
		s := base64.StdEncoding.EncodeToString(buffer)
		var trimmed bytes.Buffer
		for _, c := range s {
			switch c {
			case '=', '+', '/':
				continue
			default:
				trimmed.WriteRune(c)
			}
		}

		s = string(trimmed.Bytes())
		if len(s) >= length {
			return s[0:length]
		}
	}
}

var templateDir = "templates"

type SSHKey struct {
	Name      string
	PublicKey Resource
}

func (k *SSHKey) String() string {
	return fmt.Sprintf("SSHKey (name=%s)", k.Name)
}

type AWSCloud struct {
	Region      string
	S3          *s3.S3
	IAM         *iam.IAM
	EC2         *ec2.EC2
	Autoscaling *autoscaling.AutoScaling
	tags        map[string]string
}

func NewAWSCloud(region string, tags map[string]string) *AWSCloud {
	c := &AWSCloud{Region: region}
	config := aws.NewConfig().WithRegion(region)
	c.EC2 = ec2.New(session.New(), config)
	c.S3 = s3.New(session.New(), config)
	c.IAM = iam.New(session.New(), config)
	c.Autoscaling = autoscaling.New(session.New(), config)
	c.tags = tags
	return c
}

func newEc2Filter(name string, values ...string) *ec2.Filter {
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
			newEc2Filter("resource-id", resourceId),
			newEc2Filter("resource-type", resourceType),
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

func (c *AWSCloud) BuildFilters() []*ec2.Filter {
	filters := []*ec2.Filter{}
	for name, value := range c.tags {
		filter := newEc2Filter("tag:"+name, value)
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

type HasId interface {
	Prefix() string
}

func (k *SSHKey) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	request := &ec2.DescribeKeyPairsInput{
		KeyNames: []*string{
			aws.String(k.Name),
		},
	}

	response, err := cloud.EC2.DescribeKeyPairs(request)
	if awsErr, ok := err.(awserr.Error); ok {
		if awsErr.Code() == "InvalidKeyPair.NotFound" {
			err = nil
		}
	}
	if err != nil {
		return fmt.Errorf("error listing keys: %v", err)
	}
	found := false
	if response != nil && len(response.KeyPairs) != 0 {
		// TODO: Check key actually matches?
		glog.V(2).Info("found AWS SSH key with name: ", k.Name)
		found = true
	}

	if found {
		return nil
	}

	file, err := output.AddResource(k.PublicKey)
	if err != nil {
		return err
	}
	output.AddEC2Command("import-key-pair", "--key-name", k.Name, "--public-key-material", "file://"+file)
	return nil
}
