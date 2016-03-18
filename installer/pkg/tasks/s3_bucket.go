package tasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/golang/glog"
	"k8s.io/contrib/installer/pkg/fi"
)

type S3Bucket struct {
	fi.SimpleUnit

	Name   *string
	Region *string

	//rendered bool
	//exists   bool
}

type S3BucketRenderer interface {
	RenderS3Bucket(actual, expected, changes *S3Bucket) error
}

func (s *S3Bucket) Prefix() string {
	return "S3Bucket"
}

func (e*S3Bucket) findRegionIfExists(c *fi.RunContext) (string, bool, error) {
	cloud := c.Cloud().(*fi.AWSCloud)

	request := &s3.GetBucketLocationInput{
		Bucket: e.Name,
	}

	response, err := cloud.GetS3(cloud.Region).GetBucketLocation(request)
	if err != nil {
		if awsError, ok := err.(awserr.Error); ok {
			if awsError.Code() == "NoSuchBucket" {
				return "", false, nil
			}
		}
		return "", false, fmt.Errorf("error getting bucket location: %v", err)
	}

	var region string
	if response.LocationConstraint == nil {
		// US Classic does not return a region
		region = "us-east-1"
	} else {
		region = *response.LocationConstraint
		// Another special case: "EU" can mean eu-west-1
		if region == "EU" {
			region = "eu-west-1"
		}
	}
	return region, true, nil
}

func (e *S3Bucket) find(c *fi.RunContext) (*S3Bucket, error) {
	region, exists, err := e.findRegionIfExists(c)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	glog.V(2).Info("found existing S3 bucket")
	actual := &S3Bucket{}
	actual.Name = e.Name
	actual.Region = &region
	return actual, nil
}

func (e *S3Bucket) Run(c *fi.RunContext) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &S3Bucket{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	err = e.checkChanges(a, e, changes)
	if err != nil {
		return err
	}

	target := c.Target.(S3BucketRenderer)
	return target.RenderS3Bucket(a, e, changes)
}

func (s *S3Bucket) checkChanges(a, e, changes *S3Bucket) error {
	if a != nil {
		if e.Name == nil {
			return MissingValueError("Name is required when creating S3Bucket")
		}
		if changes.Region != nil {
			return InvalidChangeError("Cannot change region of existing S3Bucket", a.Region, e.Region)
		}
	}
	return nil
}

func (t *AWSAPITarget) RenderS3Bucket(a, e, changes *S3Bucket) error {
	if a == nil {
		glog.V(2).Infof("Creating S3Bucket with Name:%q", *e.Name)

		request := &s3.CreateBucketInput{}
		request.Bucket = e.Name

		_, err := t.cloud.GetS3(*e.Region).CreateBucket(request)
		if err != nil {
			return fmt.Errorf("error creating S3Bucket: %v", err)
		}
	}

	return nil //return output.AddAWSTags(cloud.Tags(), v, "vpc")
}

func (t *BashTarget) RenderS3Bucket(a, e, changes *S3Bucket) error {
	if a == nil {
		glog.V(2).Infof("Creating S3Bucket with Name:%q", *e.Name)

		args := []string{"mb"}
		args = append(args, "s3://" + *e.Name)

		t.AddS3Command(*e.Region, args...)
	}

	return nil
}

//func (b *S3Bucket) Region() string {
//	if !b.rendered {
//		glog.Fatalf("not yet rendered")
//	}
//	return b.region
//}

//func (b *S3Bucket) exists() bool {
//	if !b.rendered {
//		glog.Fatalf("not yet rendered")
//	}
//	return b.exists
//}
