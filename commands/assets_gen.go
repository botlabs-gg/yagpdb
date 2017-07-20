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
		size:    11319,
		modtime: 1500565928,
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
