package files

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/kubernetes/contrib/installer/pkg/fi"
)

const defaultMode = 0644

func New() *File {
	return &File{
		Mode: defaultMode,
	}
}

func Path(path string) *File {
	f := New()
	f.Path = path
	return f
}

func (f *File) WithContents(contents fi.ContentsFunction) *File {
	f.Contents = contents
	return f
}

func (f *File) WithMode(mode os.FileMode) *File {
	f.Mode = mode
	return f
}

func (f *File) DoTouch() *File {
	f.Touch = true
	return f
}

type File struct {
	Path     string
	Mode     os.FileMode
	Contents fi.ContentsFunction
	Touch    bool
}

func (f *File) Configure(c *fi.Context) error {
	if f.Touch {
		// TODO: Only write if not exists
		// also handle empty Contents
		panic("touch not implemented")
	}

	data, err := f.Contents()
	if err != nil {
		return fmt.Errorf("error building contents for file %q: %v", f.Path, err)
	}

	// TODO: Only write if changed

	mode := f.Mode
	if mode == 0 {
		mode = defaultMode
	}

	err = ioutil.WriteFile(f.Path, []byte(data), mode)
	if err != nil {
		return fmt.Errorf("error writing file %q: %v", f.Path, err)
	}

	return nil
}
