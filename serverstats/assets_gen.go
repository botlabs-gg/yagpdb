package serverstats

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

	"/assets/schema.sql": {
		local:   "assets/schema.sql",
		size:    376,
		modtime: 1506183719,
		compressed: `
H4sIAAAAAAAA/3yPwUrDQBCGzztPMccW8gY9RV0hGKOkEdrTsrpDHUh3y86siE8vNalFqL1+fPwz321v
68HiUN+0Fpt77J4GtJtmPaxRKH9QdqJexR0ocwqCCzAc8JV3Qpn9+ON3L22Lz33zWPdbfLDbCgwYUZ+V
AqLynkT9/qBfFZhQsldO8TjBUSsAsys8BscB8ReaIpQndGZv7z5GGt10f2apRMWzBssVwNzUdHd2c7Hp
T5I73f/EFC9GL07GcnV1eyp2/+/MxvHH7wAAAP//TnSGKXgBAAA=
`,
	},

	"/assets/serverstats.html": {
		local:   "assets/serverstats.html",
		size:    7161,
		modtime: 1506183349,
		compressed: `
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
d7tUCI3S/XI3aQP9nwAAAP//vv6SAPkbAAA=
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
