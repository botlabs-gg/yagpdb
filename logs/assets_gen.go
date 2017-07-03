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
		size:    5236,
		modtime: 1499120955,
		compressed: `
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
AA==
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
