package filestore

import "k8s.io/contrib/installer/pkg/fi"

type FileStore interface {
	PutResource(key string, resource fi.Resource, hashAlgorithm fi.HashAlgorithm) (url string, hash string, err error)
}