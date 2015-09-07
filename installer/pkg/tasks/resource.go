package tasks

import "io"

type Resource interface {
}

type FileResource struct {
	Path string
}

type DynamicResource interface {
	Prefix() string
	Write(w io.Writer) error
}
