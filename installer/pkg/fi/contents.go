package fi

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

/*
type FileSystemResources struct {
	fs http.FileSystem
}

func (r *FileSystemResources) Get(key string) (Resource, bool) {
	useLocal := false // TODO: Allow?
	fs := fsFunc(useLocal)
	// TODO: Check exists
	file, err := fs.Open(path)
	if err != nil {
		glog.Warningf("error opening file %q: %v", path, err)
		return nil, false
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("error reading file %q: %v", path, err)
	}
	return string(b), nil
}
*/

type FileSystemResource struct {
	fs   http.FileSystem
	path string
}

func (r *FileSystemResource) WriteTo(out io.Writer) error {
	in, err := r.open()
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return fmt.Errorf("error copying resource %q: %v", r.path, err)
	}

	return nil
}

func (r *FileSystemResource) SameContents(path string) (bool, error) {
	in, err := r.open()
	if err != nil {
		return false, err
	}
	defer in.Close()

	data, err := ioutil.ReadAll(in)
	if err != nil {
		return false, fmt.Errorf("error reading resource %q: %v", r.path, err)
	}

	return HasContents(path, data)
}

func (r *FileSystemResource) open() (http.File, error) {
	in, err := r.fs.Open(r.path)
	if err != nil {
		return nil, fmt.Errorf("error opening resource %q: %v", r.path, err)
	}
	return in, nil
}

func EmbeddedResource(fs http.FileSystem, path string) Resource {
	return &FileSystemResource{
		fs:   fs,
		path: path,
	}
}
