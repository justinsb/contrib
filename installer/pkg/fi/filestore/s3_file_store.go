package filestore

import (
	"k8s.io/contrib/installer/pkg/fi"
	fi_s3 "k8s.io/contrib/installer/pkg/fi/aws/s3"
)

type S3FileStore struct {
	bucket *fi_s3.S3Bucket
	prefix string
}

func NewS3FileStore(bucket *fi_s3.S3Bucket, prefix string) *S3FileStore {
	return &S3FileStore{
		bucket: bucket,
		prefix: prefix,
	}
}

func (s*S3FileStore) PutResource(key string, r fi.Resource) (string, string, error) {
	hash, err := fi.HashMD5ForResource(r)
	if err != nil {
		return "", "", err
	}

	s3key := s.prefix + key + "-" + hash
	o, err := s.bucket.FindObjectIfExists(s3key)
	if err != nil {
		return "", "", err
	}

	alreadyPresent := false

	s3hash := ""
	if o != nil {
		s3hash, err = o.Etag()
		if err != nil {
			return "", "", err
		}
		if s3hash == hash {
			alreadyPresent = true
		}
	}

	if !alreadyPresent {
		body, err := r.Open()
		if err != nil {
			return "", "", err
		}
		defer fi.SafeClose(body)

		o, err = s.bucket.PutObject(key, body)
		if err != nil {
			return "", "", err
		}
		s3hash = hash
	}

	err = o.SetPublicACL()
	if err != nil {
		return "", "", err
	}

	return o.PublicURL(), s3hash, nil
}