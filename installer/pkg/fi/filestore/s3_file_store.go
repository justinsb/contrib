package filestore

import (
	"k8s.io/contrib/installer/pkg/fi"
	fi_s3 "k8s.io/contrib/installer/pkg/fi/aws/s3"
"github.com/golang/glog"
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

func (s*S3FileStore) PutResource(key string, r fi.Resource, hashAlgorithm fi.HashAlgorithm) (string, string, error) {
	hashes, err := fi.HashesForResource(r, []fi.HashAlgorithm{ fi.HashAlgorithmMD5, hashAlgorithm })
	if err != nil {
		return "", "", err
	}

	md5 := hashes[fi.HashAlgorithmMD5]
	userHash := hashes[hashAlgorithm]

	s3key := s.prefix + key + "-" + userHash
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
		if s3hash == md5 {
			alreadyPresent = true
		} else {
			glog.Infof("Found file, but did not match: %q (%s vs %s)", o, s3hash, md5)
		}
	}

	if !alreadyPresent {
		body, err := r.Open()
		if err != nil {
			return "", "", err
		}
		defer fi.SafeClose(body)

		o, err = s.bucket.PutObject(s3key, body)
		if err != nil {
			return "", "", err
		}
	}

	isPublic, err := o.IsPublic()
	if err != nil {
		return "", "", err
	}
	if !isPublic {
		err = o.SetPublicACL()
		if err != nil {
			return "", "", err
		}
	}

	return o.PublicURL(), userHash, nil
}