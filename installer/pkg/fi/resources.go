package fi

import "io"

type Resources interface {
	Get(key string) (Resource, bool)
}

type Resource interface {
	WriteTo(io.Writer) error
	SameContents(path string) (bool, error)
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

func NewStringResource(s string) *StringResource {
	return &StringResource{
		s: s,
	}
}

func (s *StringResource) WriteTo(out io.Writer) error {
	_, err := out.Write([]byte(s.s))
	return err
}

func (s *StringResource) SameContents(path string) (bool, error) {
	return HasContents(path, []byte(s.s))
}
