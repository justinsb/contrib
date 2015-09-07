package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/golang/glog"
)

type S3Bucket struct {
	Name         string
	CreateRegion string

	rendered bool
	region   string
	exists   bool
}

func (b *S3Bucket) String() string {
	return fmt.Sprintf("S3Bucket  (name=%s)", b.Name)
}

func (b *S3Bucket) Region() string {
	if !b.rendered {
		glog.Fatalf("not yet rendered")
	}
	return b.region
}

func (b *S3Bucket) Exists() bool {
	if !b.rendered {
		glog.Fatalf("not yet rendered")
	}
	return b.exists
}

func (b *S3Bucket) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	var existing *s3.GetBucketLocationOutput

	name := b.Name

	request := &s3.GetBucketLocationInput{
		Bucket: aws.String(name),
	}

	response, err := cloud.s3.GetBucketLocation(request)
	if err != nil {
		if awsError, ok := err.(awserr.Error); ok {
			if awsError.Code() == "NoSuchBucket" {
				err = nil
				response = nil
			}
		}
	}
	if err != nil {
		return fmt.Errorf("error getting bucket location: %v", err)
	}

	if response != nil && response.LocationConstraint != nil {
		glog.V(2).Info("found existing S3 bucket")
		existing = response
	}

	region := b.CreateRegion
	if existing == nil {
		glog.V(2).Info("s3 bucket not found; will create: ", b)
		args := []string{"mb"}
		args = append(args, "s3://"+name)

		output.AddS3Command(b.CreateRegion, args...)
	} else {
		region = aws.StringValue(existing.LocationConstraint)
	}

	b.rendered = true
	b.exists = existing != nil
	b.region = region

	return nil
}
