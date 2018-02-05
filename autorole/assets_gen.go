package autorole

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
		size:    3135,
		modtime: 1517699158,
		compressed: `
H4sIAAAAAAAA/8xXS2/jNhC++1dMeekuUFlIj4UtIOgutlsgTbHNvaCkkcSG4rDkyK4h6L8XpB6xHTvZ
PhbYHGKH8+DM980MJ31fYqUMgijs77JjcqRRDMNq1feMrdWSR1mDshSwHobVplQ7KLT0fisc7UW2AgA4
Pi1IJ7pObr6fZFHe3MxiK2tMgj90Irud7tykzc2Rts0+qB0CdQxB6iHE1kpWhdT6ABU5aLHN0flNaqcI
0lLtpq/fJAmk6yUOSJJsNcnP8pIaHfsps9HM0X40qMi10CI3VG6FJc8CZMGKzFakrTSyxrTv17cFqx1+
6JQu1x/fDUO6wPgcmSe8Pgezcx0rDWqIv5MSK9lpPtO+aJHkVB4uKE5A/9hIU6OHVh6A5SNCZ4EJJLTK
dIyQY0UOgRs8QCN3CNIcAKsKC16wfymGgGJSO+rslRiigZY56sDrVsz4JSOItyfMS+9VbWbygRvlYayf
6OGFGzxqLBhUeX7DSaAFGXakBRjZ4lZ8euLx2k/fBy/3NhSGh5Ny+BRLd/2Tqhv0HP6C9VzwUQjiFzII
b0rlZa6xfBta72oG6ZjCFcifyv/LsFF2ToYcRXYXC8MDVTMPjbLg8M9OOSxjc34eJcrYjoEPFrfCdMHT
FTJOSFsCmTmaLn63nFstC2xIl+i2QsBO6g63IvTqAv6ZzTBca5D/HdZsuhsC81SFxvI4TTkmyBGsQ4+G
gUwQThjPaG5yl75e5afIxDpcoG07zWrSO4U5SqzGSceG5gjQT9TGdvH/pB3ugptLPWGUhudsRNlX1AHZ
x9qE2WeRrEbYK24iIxVpTXtl6pG2f0PN6Pm/EqOily/Gy1GQXwcrxw9mg8VjTn+9kvbmlRm06B3PosX3
RNa90Yd78zMpI6DvVXUE0ZNoGKIZln2PphyGDIJsfrCCMuwbNOND+gcp8x2UBIYY6rDqKIZcFo/hU/G3
Hhy2tAvT1FEbbFqQFaPbS1f6lzN+dey+wMgouroqdM6h4SWruQPC3NoUVGLW9+tfHRXovTL1MGzSeDo/
E2t4/3D7wyXV9w+3i3Y7PS9v7l7dSCAnBs/SsT8Lyb+9vpzY7IHAM1nwyNHNnER8jsVFywuoXTrKO+Yw
t2Md+S5vFS/9nbOBnE1inWqlO8Tvuo4fuabiUUD2m9zhJh2dZABw6nzcT8c1MGyoVwK5uP7CxRV52nWP
b1lt0tCMzzbliojDA70e/zuINf53AAAA///ojtsmPwwAAA==
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
