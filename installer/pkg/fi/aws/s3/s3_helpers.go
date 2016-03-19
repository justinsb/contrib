package s3

import (
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"fmt"
	"github.com/golang/glog"
	"github.com/aws/aws-sdk-go/aws/session"
	"io"
)

const (
	aclAllUsers = "http://acs.amazonaws.com/groups/global/AllUsers"
)

type S3Helper struct {
	config    *aws.Config
	defaultS3 *s3.S3
	regions   map[string]*s3.S3
}

func NewS3Helper(defaultConfig *aws.Config) *S3Helper {
	s := &S3Helper{
		config: defaultConfig.Copy(),
		defaultS3: s3.New(session.New(), defaultConfig),
		regions: make(map[string]*s3.S3),
	}

	return s
}

func (s*S3Helper) GetS3(region string) *s3.S3 {
	client, found := s.regions[region]
	if !found {
		config := s.config.Copy().WithRegion(region)
		client = s3.New(session.New(), config)
		s.regions[region] = client
	}
	return client
}

func (s*S3Helper) FindBucketIfExists(name string) (*S3Bucket, error) {
	request := &s3.GetBucketLocationInput{
		Bucket: aws.String(name),
	}

	response, err := s.defaultS3.GetBucketLocation(request)
	if err != nil {
		if awsError, ok := err.(awserr.Error); ok {
			if awsError.Code() == "NoSuchBucket" {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("error getting bucket location: %v", err)
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

	bucket := &S3Bucket{
		region: region,
		s3: s.GetS3(region),
		Name: name,
	}
	return bucket, nil
}

func (s*S3Helper) EnsureBucket(name string, region string) (*S3Bucket, error) {
	bucket, err := s.FindBucketIfExists(name)
	if err != nil {
		return nil, err
	}
	if bucket == nil {
		request := &s3.CreateBucketInput{
			Bucket: aws.String(name),
		}
		client := s.GetS3(region)
		_, err := client.CreateBucket(request)
		if err != nil {
			return nil, fmt.Errorf("error creating bucket: %v", err)
		}
		bucket = &S3Bucket{
			region: region,
			s3: client,
			Name : name,
		}
	}
	return bucket, nil
}

type S3Bucket struct {
	region string
	s3     *s3.S3
	Name   string
}

func (b*S3Bucket) Region() string {
	return b.region
}

func (b*S3Bucket) FindObjectIfExists(key string) (*S3Object, error) {
	o := &S3Object{
		Bucket: b,
		Key: key,
	}
	response, err := o.headObject()
	if err != nil {
		return nil, err
	}
	o.etag = response.ETag
	return o, nil
}

func (b*S3Bucket) PublicURL() (string) {
	if b.region == "us-east-1" {
		return "https://s3.amazonaws.com/"
	} else if b.region == "cn-north-1" {
		return "https://s3.cn-north-1.amazonaws.com.cn/"
	} else {
		return "https://s3-" + b.region + ".amazonaws.com/"
	}
}

func (b*S3Bucket) PutObject(key string, body io.ReadSeeker) (*S3Object, error) {
	o := &S3Object{
		Bucket: b,
		Key: key,
	}
	response, err := o.putObject(body)
	if err != nil {
		return nil, err
	}
	o.etag = response.ETag
	return o, nil
}

func (o*S3Object) putObject(body io.ReadSeeker) (*s3.PutObjectOutput, error) {
	request := &s3.PutObjectInput{
		Bucket: aws.String(o.Bucket.Name),
		Key:    aws.String(o.Key),
		Body: body,
	}
	response, err := o.Bucket.s3.PutObject(request)
	if err != nil {
		return nil, fmt.Errorf("error uploading S3 object %q: %v", o.friendlyName(), err)
	}

	return response, nil
}

type S3Object struct {
	Bucket *S3Bucket
	Key    string
	etag   *string
}

func (o*S3Object) headObject() (*s3.HeadObjectOutput, error) {
	request := &s3.HeadObjectInput{
		Bucket: aws.String(o.Bucket.Name),
		Key:    aws.String(o.Key),
	}

	response, err := o.Bucket.s3.HeadObject(request)
	if err != nil {
		if requestFailure, ok := err.(awserr.RequestFailure); ok {
			if requestFailure.StatusCode() == 404 {
				glog.V(4).Infof("S3 file does not exist: %q", o.friendlyName())
				return nil, nil
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("error getting S3 metadata for %q: %v", o.friendlyName(), err)
	}
	return response, nil
}

func (o*S3Object) friendlyName() string {
	return fmt.Sprintf("s3://%s/%s", o.Bucket.Name, o.Key)
}

func (o*S3Object) IsPublic() (bool, error) {
	aclRequest := &s3.GetObjectAclInput{
		Bucket: aws.String(o.Bucket.Name),
		Key: aws.String(o.Key),
	}
	aclResponse, err := o.Bucket.s3.GetObjectAcl(aclRequest)
	if err != nil {
		return false, fmt.Errorf("error getting S3 ACL for %q: %v", o.friendlyName(), err)
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
		if grantee == aclAllUsers {
			isPublic = true
		}
	}
	return isPublic, nil
}

func (o*S3Object) Etag() (string, error) {
	return *o.etag, nil
}

func (o*S3Object) PublicURL() (string) {
	bucketBase := o.Bucket.PublicURL()
	return bucketBase + o.Key
}

func (o*S3Object) SetPublicACL() (error) {
	request := &s3.PutObjectAclInput{
		Bucket: aws.String(o.Bucket.Name),
		Key: aws.String(o.Key),
		GrantRead: aws.String(aclAllUsers),
	}
	_, err := o.Bucket.s3.PutObjectAcl(request)
	if err != nil {
		return fmt.Errorf("error setting S3 ACL for %q: %v", o.friendlyName(), err)
	}

	return nil
}
