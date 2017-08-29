package commands

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

	"/assets/commands.html": {
		local:   "assets/commands.html",
		size:    11049,
		modtime: 1502705242,
		compressed: `
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
uSkrAAA=
`,
	},

	"/assets/help-page.md": {
		local:   "assets/help-page.md",
		size:    4394,
		modtime: 1504009880,
		compressed: `
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
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
