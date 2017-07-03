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
		size:    7471,
		modtime: 1490711498,
		compressed: `
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
emUvHQAA
`,
	},

	"/assets/help-page.md": {
		local:   "assets/help-page.md",
		size:    9,
		modtime: 1490711351,
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
