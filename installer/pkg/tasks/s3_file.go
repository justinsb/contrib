package tasks

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
	"k8s.io/contrib/installer/pkg/fi"
)

type S3File struct {
	fi.SimpleUnit

	Bucket    *S3Bucket
	Key       *string
	Source    Resource
	Public    *bool

	//rendered  bool
	publicURL *string

	etag      *string
}

type S3FileRenderer interface {
	RenderS3File(actual, expected, changes *S3File) error
}

func (s *S3File) Prefix() string {
	return "S3File"
}

func (e *S3File) find(c *fi.RunContext) (*S3File, error) {
	cloud := c.Cloud().(*fi.AWSCloud)

	region, exists, err := e.Bucket.findRegionIfExists(c)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	request := &s3.HeadObjectInput{
		Bucket: e.Bucket.Name,
		Key:    e.Key,
	}

	response, err := cloud.GetS3(region).HeadObject(request)
	if err != nil {
		if requestFailure, ok := err.(awserr.RequestFailure); ok {
			if requestFailure.StatusCode() == 404 {
				glog.V(2).Infof("S3 file does not exist: s3://%s/%s", *e.Bucket.Name, e.Key)
				return nil, nil
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("error getting S3 file metadata: %v", err)
	}

	aclRequest := &s3.GetObjectAclInput{
		Bucket: e.Bucket.Name,
		Key: e.Key,
	}
	aclResponse, err := cloud.GetS3(region).GetObjectAcl(aclRequest)
	if err != nil {
		return nil, fmt.Errorf("error getting S3 file ACL: %v", err)
	}

	isPublic := false
	for _, grant := range aclResponse.Grants {
		if grant.Grantee == nil {
			continue
		}
		grantee := aws.StringValue(grant.Grantee.URI)
		permission := aws.StringValue(grant.Permission)
		glog.Infof("permission:%q grant:%q", permission, grantee)
		if permission != "READ" {
			continue
		}
		if grantee == "http://acs.amazonaws.com/groups/global/AllUsers" {
			isPublic = true
		}
	}

	actual := &S3File{}
	actual.Public = &isPublic
	actual.Bucket = e.Bucket
	actual.Key = e.Key
	actual.etag = response.ETag
	return actual, nil
}

func (e *S3File) Run(c *fi.RunContext) error {
	a, err := e.find(c)
	if err != nil {
		return err
	}

	changes := &S3File{}
	changed := BuildChanges(a, e, changes)
	if !changed {
		return nil
	}

	err = e.checkChanges(a, e, changes)
	if err != nil {
		return err
	}

	target := c.Target.(S3FileRenderer)
	return target.RenderS3File(a, e, changes)
}

func (s *S3File) checkChanges(a, e, changes *S3File) error {
	if a != nil {
		if e.Key == nil {
			return MissingValueError("Key is required when creating S3File")
		}
	}
	return nil
}

func (t *AWSAPITarget) RenderS3File(a, e, changes *S3File) error {
	panic("S3 Render to AWSAPITarget not implemented")
	//if a == nil {
	//	glog.V(2).Infof("Creating S3File with Name:%q", *e.Name)
	//
	//	request := &s3.CreateBucketInput{}
	//	request.Bucket = e.Name
	//
	//	_, err := t.cloud.GetS3(*e.Region).CreateBucket(request)
	//	if err != nil {
	//		return fmt.Errorf("error creating S3File: %v", err)
	//	}
	//}
	//
	//return nil //return output.AddAWSTags(cloud.Tags(), v, "vpc")
}

func (t *BashTarget) RenderS3File(a, e, changes *S3File) error {
	needToUpload := true

	localPath, err := t.AddResource(e.Source)
	if err != nil {
		return err
	}

	if e.Bucket.Region == nil {
		panic("Bucket region not set")
	}
	region := *e.Bucket.Region

	if a != nil {
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
		s3Hash := aws.StringValue(a.etag)
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
		args = append(args, "--bucket", *e.Bucket.Name)
		args = append(args, "--key", *e.Key)
		args = append(args, "--body", localPath)
		t.AddS3APICommand(region, args...)
	}

	publicURL := ""
	if changes.Public != nil {
		// TODO: Check existing?

		if !*changes.Public {
			panic("Only change to make S3File public is implemented")
		}

		args := []string{"put-object-acl"}
		args = append(args, "--bucket", *e.Bucket.Name)
		args = append(args, "--key", *e.Key)
		args = append(args, "--grant-read", "uri=\"http://acs.amazonaws.com/groups/global/AllUsers\"")
		t.AddS3APICommand(region, args...)

		publicURLBase := "https://s3-" + region + ".amazonaws.com"
		if region == "us-east-1" {
			// US Classic does not follow the pattern
			publicURLBase = "https://s3.amazonaws.com"
		}

		publicURL = publicURLBase + "/" + *e.Bucket.Name + "/" + *e.Key
	}

	//e.rendered = true
	e.publicURL = &publicURL

	return nil
}

func (f *S3File) PublicURL() string {
	if f.publicURL == nil {
		panic("S3File not rendered or not public")
	}
	return *f.publicURL
}

func (f *S3File) String() string {
	return fmt.Sprintf("S3File (s3://%s/%s)", *f.Bucket.Name, *f.Key)
}


