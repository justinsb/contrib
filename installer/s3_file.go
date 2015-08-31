package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

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
			if requestFailure, ok := err.(awserr.RequestFailure); ok {
				if requestFailure.StatusCode() == 404 {
					err = nil
					response = nil
				}
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

	localPath, err := output.AddResource(f.Source)
	if err != nil {
		return err
	}

	if existing != nil {
		hasher := md5.New()
		f, err := os.Open(localPath)
		if err != nil {
			return err
		}
		defer func() {
			err := f.Close()
			if err != nil {
				glog.Warning("error closing local resource: ", err)
			}
		}()
		if _, err := io.Copy(hasher, f); err != nil {
			return fmt.Errorf("error while hashing local file: %v", err)
		}
		localHash := hex.EncodeToString(hasher.Sum(nil))
		s3Hash := aws.StringValue(existing.ETag)
		s3Hash = strings.Replace(s3Hash, "\"", "", -1)
		if localHash == s3Hash {
			glog.V(2).Info("s3 files match; skipping upload")
			needToUpload = false
		} else {
			glog.V(2).Infof("s3 file mismatch; will upload (%s vs %s)", localHash, s3Hash)
		}
	}
	if needToUpload {
		// We use put-object instead of cp so that we don't do multipart, so the etag is the simple md5
		args := []string{"put-object"}
		args = append(args, "--bucket", f.Bucket.Name)
		args = append(args, "--key", f.Key)
		args = append(args, "--body", localPath)
		output.AddS3APICommand(region, args...)
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
