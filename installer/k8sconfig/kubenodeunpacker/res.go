package kubenodeunpacker

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sync"
	"time"
)

type _escLocalFS struct{}

var _escLocal _escLocalFS

type _escStaticFS struct{}

var _escStatic _escStaticFS

type _escDirectory struct {
	fs   http.FileSystem
	name string
}

type _escFile struct {
	compressed string
	size       int64
	modtime    int64
	local      string
	isDir      bool

	data []byte
	once sync.Once
	name string
}

func (_escLocalFS) Open(name string) (http.File, error) {
	f, present := _escData[path.Clean(name)]
	if !present {
		return nil, os.ErrNotExist
	}
	return os.Open(f.local)
}

func (_escStaticFS) prepare(name string) (*_escFile, error) {
	f, present := _escData[path.Clean(name)]
	if !present {
		return nil, os.ErrNotExist
	}
	var err error
	f.once.Do(func() {
		f.name = path.Base(name)
		if f.size == 0 {
			return
		}
		var gr *gzip.Reader
		b64 := base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(f.compressed))
		gr, err = gzip.NewReader(b64)
		if err != nil {
			return
		}
		f.data, err = ioutil.ReadAll(gr)
	})
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (fs _escStaticFS) Open(name string) (http.File, error) {
	f, err := fs.prepare(name)
	if err != nil {
		return nil, err
	}
	return f.File()
}

func (dir _escDirectory) Open(name string) (http.File, error) {
	return dir.fs.Open(dir.name + name)
}

func (f *_escFile) File() (http.File, error) {
	type httpFile struct {
		*bytes.Reader
		*_escFile
	}
	return &httpFile{
		Reader:   bytes.NewReader(f.data),
		_escFile: f,
	}, nil
}

func (f *_escFile) Close() error {
	return nil
}

func (f *_escFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, nil
}

func (f *_escFile) Stat() (os.FileInfo, error) {
	return f, nil
}

func (f *_escFile) Name() string {
	return f.name
}

func (f *_escFile) Size() int64 {
	return f.size
}

func (f *_escFile) Mode() os.FileMode {
	return 0
}

func (f *_escFile) ModTime() time.Time {
	return time.Unix(f.modtime, 0)
}

func (f *_escFile) IsDir() bool {
	return f.isDir
}

func (f *_escFile) Sys() interface{} {
	return f
}

// FS returns a http.Filesystem for the embedded assets. If useLocal is true,
// the filesystem's contents are instead used.
func FS(useLocal bool) http.FileSystem {
	if useLocal {
		return _escLocal
	}
	return _escStatic
}

// Dir returns a http.Filesystem for the embedded assets on a given prefix dir.
// If useLocal is true, the filesystem's contents are instead used.
func Dir(useLocal bool, name string) http.FileSystem {
	if useLocal {
		return _escDirectory{fs: _escLocal, name: name}
	}
	return _escDirectory{fs: _escStatic, name: name}
}

// FSByte returns the named file from the embedded assets. If useLocal is
// true, the filesystem's contents are instead used.
func FSByte(useLocal bool, name string) ([]byte, error) {
	if useLocal {
		f, err := _escLocal.Open(name)
		if err != nil {
			return nil, err
		}
		return ioutil.ReadAll(f)
	}
	f, err := _escStatic.prepare(name)
	if err != nil {
		return nil, err
	}
	return f.data, nil
}

// FSMustByte is the same as FSByte, but panics if name is not present.
func FSMustByte(useLocal bool, name string) []byte {
	b, err := FSByte(useLocal, name)
	if err != nil {
		panic(err)
	}
	return b
}

// FSString is the string version of FSByte.
func FSString(useLocal bool, name string) (string, error) {
	b, err := FSByte(useLocal, name)
	return string(b), err
}

// FSMustString is the string version of FSMustByte.
func FSMustString(useLocal bool, name string) string {
	return string(FSMustByte(useLocal, name))
}

var _escData = map[string]*_escFile{

	"/kube-node-unpacker.sh": {
		local:   "res/kube-node-unpacker.sh",
		size:    1531,
		modtime: 1453050623,
		compressed: `
H4sIAAAJbogA/2xU72/bNhD9rr/i5hTDBiSSna1fFriDl6aY0MIBYndFURQDRZ0kIhSpkScrxtb/fUdK
+WG7/mBJ5PHdu8d3d/ZDViiTFcI3SXIG17bbO1U3BJfzxWvYNgjv+wKdQUIPq54a6/ipNcQoDw49uh2W
aXLGxz8oicZjCb0p0QHx8VUnJD+mnXP4C51X1sBlOoefQsBs2pr9fMUIe9tDK/ZgLEHvkSGUh0ppBHyQ
2BEoA9K2nVbCSIRBURPTTCBMAz5PELYgwdGC4zv+ql7GgaBIOPwaou63LBuGIRWRbGpdnekx0Gcf8uub
9ebmggnHIx+NRh8K/6dXjkst9iA65iNFwSy1GMA6ELVD3iMb+A5OkTL1OXhb0SAcMkqpPDlV9HQg1iM7
rvllAMslDMxWG8g3M/hjtck354zxKd/+eftxC59Wd3er9Ta/2cDtHVzfrt/m2/x2zV/vYLX+DO/z9dtz
QJaK0+BD5wJ/JqmCjPHqYIN4QKCyIyHfoVSVklyXqXtRI9R2x3bgcqBD1yofLtMzvZJRtGoVCYorJ0Wl
wV/aihLLvGWkd1rUPlQqoFB0UfFn0IuckPcwNEo2UFp5j4Enh/vpLPheSi6g6rXep4lGOgFdzpOEAfg2
yPV4xTAJBKOScPT3iLmshPaY8LqqDpMt3mQl7jLD8HD55sfFVajBhEgAUi3anuCX+WN0SA0XCjLvdpkX
mrJ7bpcLbik/vnXOPuxTznwCHBGdXL76Pb4xjy9fYPbqXye/zWC5hDl8/ToljzaF75bKB47Xvv23mMUj
qE9AF5e/HsMeCRMkizuVSuIf/589u/3wSmobGpLPa80+Ym8pglF5bW2XJi/KOmE58nliA4VDcX/1lHFj
Wwx6+wOpGxH6aKL8uFMKbLlFeNXq3ehj9mUs41mAgyo5+VPeML/YoY9gU+AzEa8Ru9gQPBA9SmtKDwXy
QpgihG0XmjtYNzI8FEjUPIIYZgRZvE6S0hoMnbC2Q9QrhVVFfGB0S+8bHhPsI55s7BOegoEaNYLGdpy4
hvlghzAVSlA8ycboURS+grBscDjiEmMKnDyUJv8HAAD//6XgAiP7BQAA
`,
	},

	"/": {
		isDir: true,
		local: "res",
	},
}
