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
		size:    12858,
		modtime: 1501193423,
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
