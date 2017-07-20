package automod

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

	"/assets/automod.html": {
		local:   "assets/automod.html",
		size:    12711,
		modtime: 1500578625,
		compressed: `
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
`,
	},

	"/assets/help-page.md": {
		local:   "assets/help-page.md",
		size:    9,
		modtime: 1499120952,
		compressed: `
H4sIAAAAAAAA/9JVCPF38VfQVQAEAAD//zTsWAgJAAAA
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
