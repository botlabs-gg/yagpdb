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
		modtime: 1499120954,
		compressed: `
H4sIAAAAAAAA/9JVCPF38VfQVQAEAAD//zTsWAgJAAAA
`,
	},

	"/assets/moderation.html": {
		local:   "assets/moderation.html",
		size:    12167,
		modtime: 1500579542,
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
