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
		size:    3350,
		modtime: 1517698822,
		compressed: `
H4sIAAAAAAAA/7xWS2/jNhC++1dMiR5aYGU1e+hhYRsIsmjRU4Gm94LSjCRuKJLlw04g+L8XpORHbEuJ
U7c+0LRnyPnm9XG6DqkSioCV5i9LiMKz7XY26zpPrZHc95KGODKYb7ezBYo1lJI7t2RWb9hqBgBw/G+p
ZSbr7O7zIEvy5m4nNrymLN5Hlq3+SBYXeXN3pGtWfzYEPRgwMtRCwUZICUY7nxYHldUtuFD0Wg6Eghcd
LKBwpbYIjuya7CI3A74cxXrYfpdlkM/3KCHLVrNBfuI1l2S9G/zuj1m96Q98LAxHcsMVSUhrhlTxIP2R
5kXtFDah6hO9+LlHBEUbqIjw9SUHz8fvLTS+XLh0yMQ+zFAJkgjCwbfgPPiGQPGWQFdpf9D7QWnIbR6z
UlmtfNQQ/sdPQM+8NZLcF2A1b8mxT8DaIL0otfFk2TmEStt2BzbuM6GkUMSgJd9oXLJYDwx46YVWS5a3
XPGa8q6b35derOnXICTOf/u63eZDeZ/7eRqUZKe2OpgR5XRA8oIkVNou2d5ztnrcbRd5Upi4QCgTPPgX
Q0vm6dmzV/ZLrbzVkoHAYwMp4q/+MJKX1GiJZJes0RvUwrUk5Zij5wVxkxiUDVeKJFs9ptaDh/7322Fw
JKn0yc3dHZcD0Xu+10HueWbp7yAsGbKty6gtCMctxU/s8Gc/YPvdxKJx8KpUBpnbbsch5z3m6wNcBO+1
GnLuQtGKQ9YLr6Dw6kAG94iLvD9xoTfzGJzVVK+f/vzPyOchWEvKH/HxzSio676vY1rgyxLOOvqS9lAe
7uzARF67znJVE8z75+hBq0rUF/SOUW9ISohL5tqxTvswd/U+7ykrsllisGCQe5pqyFFOaQQiKbjUUQIZ
rLkMtGSDoSkL13IE3IIrp337F3x57PljKKZdH2/sm0Rmz6DvpU743+kTxij00He7RpvgT3iTQ+Ed4Y4D
2S+hfBKqhsa3ErSSL8Cl1BsHNXngCvuRkTsMQn/jDg+rRgG+ES5OM59/uvs5pgCqUD45cPyJ0pA3avo9
NO5CWZJzLLXc9S3+yNc0zv5XIcFIbvZaIEiSIpCv6XsayqXHaCKFXUcKT+rjjbern777Z2uXmncO9SeT
++mQX2kdJ8805M8GZP8EAAD//6X3mqsWDQAA
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
