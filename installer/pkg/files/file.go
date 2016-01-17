package files

import (
	"fmt"
	"os"

	"github.com/golang/glog"
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

func (f *File) WithContents(contents fi.Resource) *File {
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
	fi.SystemUnit

	Path     string
	Mode     os.FileMode
	Contents fi.Resource
	Touch    bool
}

func (f *File) Run(c *fi.RunContext) error {
	exists := Exists(f.Path)
	if f.Touch {
		// TODO: handle empty Contents

		if exists {
			glog.Warningf("TODO: Verify mode")
			glog.V(2).Infof("File exists; won't touch: %q", f.Path)
			return nil
		}
	}

	if exists {
		same, err := f.Contents.SameContents(f.Path)
		if err != nil {
			return fmt.Errorf("error checking contents of %q: %v", f.Path, err)
		}
		if same {
			glog.Warningf("TODO: Verify mode")
			return nil
		}
	}

	if c.IsConfigure() {
		mode := f.Mode
		if mode == 0 {
			mode = defaultMode
		}

		out, err := os.OpenFile(f.Path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
		if err != nil {
			return fmt.Errorf("error opening file for write %q: %v", f.Path, err)
		}
		defer out.Close()

		err = f.Contents.WriteTo(out)
		if err != nil {
			return fmt.Errorf("error writing file %q: %v", f.Path, err)
		}

		return nil
	} else if c.IsValidate() {
		c.MarkDirty()
		return nil
	} else {
		panic("Unhandled run action")
	}
}
