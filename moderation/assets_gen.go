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
		size:    12347,
		modtime: 1499639150,
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
