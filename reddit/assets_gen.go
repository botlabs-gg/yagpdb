package reddit

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

	once sync.Once
	data []byte
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
		b, err := ioutil.ReadAll(f)
		f.Close()
		return b, err
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

	"/assets/settings.html": {
		local:   "assets/settings.html",
		size:    3485,
		modtime: 1500655830,
		compressed: `
H4sIAAAAAAAA/7RWS2/jNhC++1dMiR5aYGU1eyjQQBYQZNFiLy3Q9F5Q4khiMiIFPuwYgv97QUp+xLGU
eJv1QabE4cw3r4/T9wIrqRBY2f1rUAjp2G63WPS9w7Yj7oadBrlgsNztFpmQayiJW7tiRm9YvgAAOP1a
akqoTm4+j3txv7nZb3e8xiToQ8Pyv6PFLG1uTmS7/J8GYQADHflaKthIIui0dfFhoTK6BeuLQcqCVLDV
3oCQttRGgEWzRpOl3Qu9WZH/qZ0s8TZLixyyUgvM4TfXwKOnbZbG91v4GoxLAe4Io0IUFpwGqUryAoE1
UghUA5DUpJyInQD6BLIKiEBFe1FVoR200lqpauBqO3rSEXKLQOigRXhSerM8wM5SIdfj8ockgXR5CC4k
Sb4Y98+SxQmNs2O6hmNGb4YD35a9k/2OKySIz0RgxT25E8mL0jHbUtVncuF3JwQo3MTwvlRy9Hxab6HF
9oLSsYAOyYBKIgmQFh69dTEVircIuorro9xPSodUhmKqjFYuSEj38yfAZ952hPYWWM1btOwTsNaTk6Xu
HBr2GkKlTbsHG9aJVCQVMmjRNVqsWEg+A146qdWKpS1XvMa075d3pZNr/MNLEsuvX3a7dOzK136eByXa
qY323YRwPEC8QIJKmxU7eM7yh/0yS6PAjAKpOu/AbTtcMYfPjr2wX2rljCYGUpwaiBF/8aEjXmKjSaBZ
sUZvhJa2RaIpR18XxIfEoGy4Ukgsf4iMAffD+9thsEhYuujmXsflQAyeH+xMagy/0MnPbsTwVxeKw8KL
khj37G43DS0dsF0fyMI7p9WYW+uLVh6zWzgFhVPHpr8TIkuHExd6MA1ByOd6+vz1u5HMvTcGlTth5w+j
mr7/sQ5pgdsVvOrcS9JjGdhXB2by2veGqxphOdyW91pVsr4gd4p6g0QQHoltpzrqmzlq8PlATYG1IlP5
TnCHc403yR372/RC50jBYM3J44qNhuYsXMsF8BGcOO/b/+DFU88ffDHv+nRjf0hkDgz2XoqE70aTMEWV
x/7aN9QMT8KbXAnvCGsYsH735VOY6hrXEmhFW+BEemOhRgdciWFy5VZ4qR+5FcenFhJcI22YTj7/cvNr
CDVUvnyyYPkTxqFt0vR76Nr6skRrWWyt61v5ga9xmuWvQiICiZlrgQgkDEC+xP95KJcunZkU9j0qcVYf
b9xRwzQ9XE/71LxzSD+bxM+H9krrMEnGoX0xIvsvAAD//6Xtv8+dDQAA
`,
	},

	"/": {
		isDir: true,
		local: "",
	},

	"/assets": {
		isDir: true,
		local: "assets",
	},
}
