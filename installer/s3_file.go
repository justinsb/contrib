package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/golang/glog"
)

type S3File struct {
	Bucket *S3Bucket
	Key    string
	Source Resource
	Public bool

	rendered  bool
	publicURL string
}

func (f *S3File) PublicURL() string {
	if !f.rendered {
		glog.Fatalf("not yet rendered")
	}
	return f.publicURL
}

func (f *S3File) String() string {
	return fmt.Sprintf("S3File (s3://%s/%s)", f.Bucket.Name, f.Key)
}

func (f *S3File) RenderBash(cloud *AWSCloud, output *BashTarget) error {
	region := f.Bucket.Region()

	var existing *s3.HeadObjectOutput

	if f.Bucket.Exists() {

		request := &s3.HeadObjectInput{
			Bucket: aws.String(f.Bucket.Name),
			Key:    aws.String(f.Key),
		}

		// TODO: Target correct region
		response, err := cloud.s3.HeadObject(request)
		if err != nil {
			if awsError, ok := err.(awserr.Error); ok {
				if awsError.Code() == "NoSuchBucket" {
					err = nil
					response = nil
				}
				glog.Info("AWS Error: ", awsError.Code())
			}
		}
		if err != nil {
			return fmt.Errorf("error getting S3 file metadata: %v", err)
		}

		if response != nil {
			glog.V(2).Info("found existing S3 file")
			existing = response
		}
	}

	needToUpload := true

	if existing != nil {
		// TODO: Only upload if changed
		glog.Info("etag matching not yet implemented")
	}

	if needToUpload {
		localPath, err := output.AddResource(f.Source)
		if err != nil {
			return err
		}

		args := []string{"cp"}
		args = append(args, localPath)
		args = append(args, "s3://"+f.Bucket.Name+"/"+f.Key)
		output.AddS3Command(region, args...)
	}

	publicURL := ""
	if f.Public {
		// TODO: Check existing?

		args := []string{"put-object-acl"}
		args = append(args, "--bucket", f.Bucket.Name)
		args = append(args, "--key", f.Key)
		args = append(args, "--grant-read", "uri=\"http://acs.amazonaws.com/groups/global/AllUsers\"")
		output.AddS3APICommand(region, args...)

		publicURLBase := "https://s3-" + region + ".amazonaws.com"
		if region == "us-east-1" {
			// US Classic does not follow the pattern
			publicURLBase = "https://s3.amazonaws.com"
		}

		publicURL = publicURLBase + "/" + f.Bucket.Name + "/" + f.Key
	}

	f.rendered = true
	f.publicURL = publicURL

	return nil
}
