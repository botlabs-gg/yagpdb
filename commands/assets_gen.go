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
		size:    11311,
		modtime: 1499120955,
		compressed: `
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
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
