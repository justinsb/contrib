package fi

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

type ContentsFunction func() (string, error)

func StaticContent(s string) ContentsFunction {
	return func() (string, error) {
		return s, nil
	}
}

func Resource(key string) ContentsFunction {
	panic("Resource not yet implemented")
}

func FSResource(fsFunc func(bool) http.FileSystem, path string) ContentsFunction {
	return func() (string, error) {
		useLocal := false // TODO: Allow?
		fs := fsFunc(useLocal)
		file, err := fs.Open(path)
		if err != nil {
			return "", fmt.Errorf("error opening file %q: %v", path, err)
		}
		defer file.Close()

		b, err := ioutil.ReadAll(file)
		if err != nil {
			return "", fmt.Errorf("error reading file %q: %v", path, err)
		}
		return string(b), nil
	}
}
