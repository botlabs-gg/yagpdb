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
		size:    12849,
		modtime: 1490711686,
		compressed: `
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
`,
	},

	"/assets/help-page.md": {
		local:   "assets/help-page.md",
		size:    9,
		modtime: 1490711730,
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
