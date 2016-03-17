package tasks

import (
	"io"
	"bytes"
)

type Resource interface {
	WriteTo(w io.Writer) error
}

type FileResource struct {
	Resource
	Path string
}

type DynamicResource interface {
	Resource
	Prefix() string
}

func ResourceAsString(r Resource) (string, error) {
	buf := new(bytes.Buffer)
	err := r.WriteTo(buf)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func ResourceAsBytes(r Resource) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := r.WriteTo(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type StringResource struct {
	s string
}

var _ Resource = &StringResource{}

func NewStringResource(s string) *StringResource {
	return &StringResource{s:s}
}

func (r*StringResource) WriteTo(w io.Writer) error {
	_, err := w.Write([]byte(r.s))
	return err
}