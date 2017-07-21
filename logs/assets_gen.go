package logs

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

	"/assets/control_panel.html": {
		local:   "assets/control_panel.html",
		size:    6261,
		modtime: 1499120955,
		compressed: `
H4sIAAAAAAAA/+xZXW/bNhe+z684L99dbMBkocOuCllDlnRFgTYr0G7ArgpKPJKIUKRAUnYNQf99oEg5
dmxLWpth7bBcWJR4znM+eD5IpusYFlwikLz5IFRZclmSvr+66jqLdSOo9VMVUkZg1fdXCeMbyAU1Zk20
2pL0CgDg8GuuRCTK6NkPYW6Yr56N0w0tMXJ4qEn62otM4urZAXGTvqGSlgg1GuOeQpUGqGSwoZqr1oCy
FWqwFZelWSVxE7SIGd+E4f+iCOLVXheIovQqzD+yjQrU1gTrPJtWW8/wacYezDdUooDhN2JY0FbYA8qz
1INz3Doc07m/lyhRUwEGrXWmHwM9WH8ZO1NsdwY4KZSuQSuBa+KGBGq0lWJr0ijzWONz6A/emaMMHvtx
gvyEpcL8PlMfZ1gAEkEzFHNUAAmXTWvB7ho8QAdJa1yT3wxqNwqx+ULSTCAj0HW8gNWNkgUvV+eJ+h4G
NGRdh5L1/bwqnjNm3LgntAEXQjImmY7nQf5QLeRUeuFABxQzQCDbQxrYcluBrRC2leIGclXXVLLVMhmH
C5KZKKdCqNbCwzDislAkTbL0Tll8nsRZCq+KoAvVCEoClT5zDeoN6geFMmXddHjnBtA79Hs3K90P10eG
CAEZjhbiBiXwAnaqhdGRfADcqXYU5s08kyRnTI0XxNEM0tz015QRdzy/n82I80SfnxEy4D5hRoyQn5UR
X2iQuAZ2yOeqeVRq1TYkHXratFX7iPlZ0PxecGMhr6iUKAwUWtVHLXmRCx4bklk56jPLB5BkrbVKAqOW
RlaVpetQTKuGqa0kB5jgcEOHhZEgcJBF1uynEtO4yAl+pxotSZPYfUyT2OuzRHWApBUjzF6jGmW7yHL/
13X1+4fNilfwg2osV9J8qFthOQFyExQnsLrOLd/gy5YLtho/A7mjNRIgey8ge2Ah71BgPgCSfTK/pdog
O6AffdP3yyyP2yVhMV+MfTwPdJPROwE1NfXv3MH43fMbH9vmhsrfOW5vUaA9qdpTpJ9Qu6+FUFuosc5c
uR2qa5IrhuOGfhSUxMNXaFDX3BgXfGAVbDhugXnpY26ar7MO/2OL/2KDeqckTi77eaJPXnAMcF/oGv5d
FeDo1HeWJ3Qvv1KmzWpuT5qWafMcjRnGohwemVD5PYH0Hd2gczG8Cwe+ff8BgCf0RBK7bUI6dZx8/OoP
yv54e1iYn+jwC7lAqgt+Lmnetpng+XHvdk5254ZwsDi18FBQK0SkeVlZkg5ZIZWF1S9cG/uWltj3CYVK
Y7EmP9HCol533eoOt2hs3+9XT9JNJLi8h/0y1sOj0bymekdSx6GTmKYhlxK6lHUUnmGhNDrpvwrmpadu
NKCeO+1PXwBYt52ONJpGScM3eO4aYKA5Yrh09rd6IlVslb66TWJbTdPcaKQW2TzhdWsrpRcA+l3KAkC/
4blMmMSXDOy6b0p4vj7ear26vbAx6jpNZYmweq3KS5unGV+y9P9dN0hIYsumKbvOJTK173mNsAr+vbbL
WFfezX0P3+5fnNjv5pmdhsH5bq+5RN504RzuoxbcQR0x+bYYorfijKEkofZa/GjHDskZgQ0VLa5JcCyB
JRcgh113rOWDu4dgWpOiFcI3vtMa7zOcuVjQe+m3gXiJcHoB8lHRiJuhOMYuSvs+drUxHo1MXat3tWOm
zZ72guP5S4s3lTNDCTzTd4Ya85fKmG8ThVIW9VSXcAkHb3ZQU/lfN1jWvxfenx9fkh/KPPm/gV8mf7d+
NUbBnwEAAP//MPcbGHUYAAA=
`,
	},

	"/assets/log_view.html": {
		local:   "assets/log_view.html",
		size:    5244,
		modtime: 1500574201,
		compressed: `
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
FAAA
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
