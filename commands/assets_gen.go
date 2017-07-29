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
		modtime: 1500579475,
		compressed: `
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
uSkrAAA=
`,
	},

	"/assets/help-page.md": {
		local:   "assets/help-page.md",
		size:    4307,
		modtime: 1500391598,
		compressed: `
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
//9axpct0xAAAA==
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
