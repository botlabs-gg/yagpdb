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
		size:    996,
		modtime: 1506449968,
		compressed: `
H4sIAAAAAAAA/3RTTW/TQBC991e8I6gpLdeeqJDggLggJIQQUifesb3qesfamU3if49m7USkgks+7J33
NW+/j4xJAheyKBk9k9XCioWGOewxFznEwAoq/Hhzgzt8lZBkgI1kSDIoKCXYNQp1/qXt8V4MRi+suAGA
W9gYFRMt2DMKT3LggJjb2b46uf/r6SAF0oNqiI0IUyxFSszDbR+Tsf9yPZ85Oy0HTKxKA6+qXEVeXNSm
ZmP/JAV8omlO/IjYY5GKqtzon+/2lPFBZWLJjD3HPECtzjE8o5Npohz8YL4YO8aUQEkFayaMRGp4//Bw
VqNnb91IOfMa1RnrSAo+cVeNww584IyYu1SDEwdO/JctdbNfYvcCn3ShG4o2Z7jFU7JR6jDunEO5Zdy0
/S/oHaQgi7UBDP8Osi8yXRmICjX3XZX7mtK71opqfNGhGodt+1M1RpHE6KWAYDzNUqgsCHUty3nqp1SM
dGCYQNlQ51cAi9SinHqn+5iY8r1/lvP8t+ZQ1wWcLqntcGkL9otrLr52nbmLfeTgaD+o5JgHvTaAcw+O
22scx9iNfhNghboXDo64VWHNkKrJdg+kOJmudXfzpSZWHKMvyVo555qjjhNn0xbir6cMPhlnjYdXWCt8
kG7F2t41LUm6trWRC/9+c+9n7q9m3/4JAAD//xYSEVLkAwAA
`,
	},

	"/assets/moderation.html": {
		local:   "assets/moderation.html",
		size:    11873,
		modtime: 1502705243,
		compressed: `
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
///RoudLYS4AAA==
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
