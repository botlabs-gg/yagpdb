package reddit

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

	"/assets/settings.html": {
		local:   "assets/settings.html",
		size:    3533,
		modtime: 1501987171,
		compressed: `
H4sIAAAAAAAA/7xWTW/jNhC9+1dMiR5aILKaPRRoIAsIsmixlxZoei8ocSQxGZEqP+wYgv97QUr+iGMp
yTZdH2RKHHLezOM8Tt8LrKRCYGX3t0EhpGO73WLR9w7bjrgbZhrkgsFyt1tkQq6hJG7tihm9YfkCAOD0
a6kpoTq5/jTOxfnmej/d8RqTsB8alv8ZPWZpc31i2+V/NQgDGOjI11LBRhJBp62LDwuV0S1YXwxWFqSC
rfYGhLSlNgIsmjWaLO2e7ZsV+e/ayRJvsrTIISu1wBx+cQ08eNpmaXy/gS/BuRTgjjAqRGHBaZCqJC8Q
WCOFQDUASU3KidgJoCuQVUAEKvqLWxXaQSutlaoGrrZjJB0htwiEDlqER6U3ywPsLBVyPQ6/SxJIl4fk
QpLki3H+jCxOaJwd6RqWGb0ZFnwdeyfzHVdIEJ+JwIp7cieWF60j21LVZ3bhdysEKNzE9D7f5Bj59L6F
FtsLm44H6EAGVBJJgLTw4K2LVCjeIugqjo92PygdqAyHqTJauWAh3Y9XgE+87QjtDbCat2jZFbDWk5Ol
7hwa9hJCpU27BxvGiVQkFTJo0TVarFggnwEvndRqxdKWK15j2vfL29LJNf7mJYnll8+7XTpW5cs4z5MS
/dRG+27COC4gXiBBpc2KHSJn+f1+mKXRYGYDqTrvwG07XDGHT449819q5YwmBlKcOogZf/ahI15io0mg
WbFGb4SWtkWiqUBfHogPyUHZcKWQWH4fFQPuhvfX02CRsHQxzP0elxMxRH6wEdzxxOA/Xhrs0LQ2wbZA
Me0p/EKFP7kR2x9dODQWnh2Vcc7udtOQ0wHz+xNceOe0Gjm3vmjlkfXCKSicOorBrRBZOqy4UJtpSE4+
V+vnr/+b+Nx5Y1C5E9X+MAnq++/rQAvcrOBFRV+yHo+HfbFghte+N1zVCMvhFr3TqpL1BbtT1BskgvBI
bDtVaV+tXUPMB8kKahYVzHeCO5wryElN2d+yFypKCgZrTh5XbHQ05+G9GgEfoZXzsf0HvTyN/N4X86FP
F/aHZOagoG+VTvjm8glTEnqsu32hzegnvKqh8IZ0h4bsV18+hi6wcS2BVrQFTqQ3Fmp0wJUYOl1uhZf6
gVtxfGohwTXShm7m00/XPwcKoPLlowXLHzE2eZOu3yLj1pclWstiyb2/xO/5GqfV/11IRBA3814gAgkD
kM/xfx7KpctohsK+RyXOzscrd9fQfQ/X1p6aNzb1Z537eZNfaR06z9jkL0Zk/wYAAP//OApC180NAAA=
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
