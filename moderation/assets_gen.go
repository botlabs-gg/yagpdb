package moderation

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

	"/assets/help-page.md": {
		local:   "assets/help-page.md",
		size:    9,
		modtime: 1490712570,
		compressed: `
H4sIAAAAAAAA/9JVCPF38VfQVQAEAAD//zTsWAgJAAAA
`,
	},

	"/assets/moderation.html": {
		local:   "assets/moderation.html",
		size:    11869,
		modtime: 1490712558,
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
