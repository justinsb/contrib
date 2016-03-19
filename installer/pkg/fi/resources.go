package fi

import (
	"io"
	"bytes"
	"fmt"
	"os"
	"crypto/md5"
	"encoding/hex"
)

type Resources interface {
	Get(key string) (Resource, bool)
}

type Resource interface {
	Open() (io.ReadSeeker, error)
	//WriteTo(io.Writer) error
	//SameContents(path string) (bool, error)
}

func HashMD5ForResource(r Resource) (string, error) {
	hasher := md5.New()
	err := CopyResource(hasher, r)
	if err != nil {
		return "", fmt.Errorf("error while hashing resource: %v", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

//type DynamicResource interface {
//	Resource
//	Prefix() string
//}

func CopyResource(dest io.Writer, r Resource) error {
	in, err := r.Open()
	if err != nil {
		return fmt.Errorf("error opening resource: %v", err)
	}
	defer SafeClose(in)

	_, err = io.Copy(dest, in)
	if err != nil {
		return fmt.Errorf("error copying resource: %v", err)
	}
	return nil
}

func ResourceAsString(r Resource) (string, error) {
	buf := new(bytes.Buffer)
	err := CopyResource(buf, r)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func ResourceAsBytes(r Resource) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := CopyResource(buf, r)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type ResourcesList struct {
	resources []Resources
}

var _ Resources = &ResourcesList{}

func (l *ResourcesList) Get(key string) (Resource, bool) {
	for _, r := range l.resources {
		resource, found := r.Get(key)
		if found {
			return resource, true
		}
	}
	return nil, false
}

func (l *ResourcesList) Add(r Resources) {
	l.resources = append(l.resources, r)
}

type StringResource struct {
	s string
}

var _ Resource = &StringResource{}

func NewStringResource(s string) *StringResource {
	return &StringResource{s:s}
}

func (s *StringResource) Open() (io.ReadSeeker, error) {
	r := bytes.NewReader([]byte(s.s))
	return r, nil
}

func (s *StringResource) WriteTo(out io.Writer) error {
	_, err := out.Write([]byte(s.s))
	return err
}

func (s *StringResource) SameContents(path string) (bool, error) {
	return HasContents(path, []byte(s.s))
}

type BytesResource struct {
	data []byte
}

var _ Resource = &BytesResource{}

func NewBytesResource(data []byte) *BytesResource {
	return &BytesResource{data: data}
}

func (r *BytesResource) Open() (io.ReadSeeker, error) {
	reader := bytes.NewReader([]byte(r.data))
	return reader, nil
}

type FileResource struct {
	Path string
}

var _ Resource = &FileResource{}

func NewFileResource(path string) *FileResource {
	return &FileResource{Path:path}
}
func (r *FileResource) Open() (io.ReadSeeker, error) {
	in, err := os.Open(r.Path)
	if err != nil {
		return nil, fmt.Errorf("error opening file %q: %v", r.Path, err)
	}
	return in, err
}

func (r *FileResource) WriteTo(out io.Writer) error {
	in, err := r.Open()
	defer SafeClose(in)
	_, err = io.Copy(out, in)
	if err != nil {
		return fmt.Errorf("error copying file %q: %v", r.Path, err)
	}
	return err
}

//func (r *FileResource) SameContents(path string) (bool, error) {
//	return HasContents(path, []byte(s.s))
//}
