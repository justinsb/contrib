package filestore

import "k8s.io/contrib/installer/pkg/fi"

type FileStore interface {
	PutResource(key string, resource fi.Resource) (url string, hash string, err error)
}