package files

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/kubernetes/contrib/installer/pkg/fi"
)

type FSResources struct {
	path string
}

var _ fi.Resources = &FSResources{}

func (r *FSResources) Get(key string) (fi.Resource, bool) {
	p := path.Join(r.path, key)
	if Exists(p) {
		return &FSResource{path: p}, true
	}
	return nil, false
}

func NewResourceDir(path string) *FSResources {
	return &FSResources{
		path: path,
	}
}

type FSResource struct {
	path string
}

var _ fi.Resource = &FSResource{}

func (f *FSResource) WriteTo(out io.Writer) error {
	in, err := f.open()
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return fmt.Errorf("error copying file %q: %v", f.path, err)
	}

	return nil
}

func (r *FSResource) SameContents(path string) (bool, error) {
	if !Exists(path) {
		return false, nil
	}

	expected, err := r.open()
	if err != nil {
		return false, err
	}
	defer expected.Close()

	actual, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer actual.Close()

	return fi.ReadersEqual(expected, actual)
}

func (r *FSResource) open() (*os.File, error) {
	in, err := os.Open(r.path)
	if err != nil {
		return nil, fmt.Errorf("error opening file %q: %v", r.path, err)
	}
	return in, nil
}
