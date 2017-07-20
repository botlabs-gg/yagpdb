package customcommands

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

	"/assets/customcommands.html": {
		local:   "assets/customcommands.html",
		size:    7487,
		modtime: 1500565956,
		compressed: `
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
///fe2YVPx0AAA==
`,
	},

	"/assets/help-page.md": {
		local:   "assets/help-page.md",
		size:    9,
		modtime: 1499120953,
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
