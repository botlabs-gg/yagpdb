package docs

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

	"/templates/docs.ghtml": {
		local:   "templates/docs.ghtml",
		size:    589,
		modtime: 1490434466,
		compressed: `
H4sIAAAAAAAA/1xQz260IBA/fz7FfPSMZHtmueyem75BM9FRSdnBINoD8d0bXLRmT0OG+f1NqaXOMoFo
fTNJxkWsa6WdNRUAgEYYAnVX8SaMttA4nKar6BA6lJY7n2f3I4xW1sDdN/ODOGK0nkFPI/IJgSH47TLv
jVZYFGa3HzEuwLjIiRrPrXS0kBOm+pfPUgrIPUF9980n9jSt6wbfKHa3xwJPlNJZ/hYlhsopVUr1Bz5o
XYU5noehjUBlyiJM3BYx/V9KUPWrR5CyZFGzM9UTvOOql4ZH7ClXnFKkx+gwEohm/BoIWwF1BujWLrv/
rbIn92nbeCddLy/v4mR5uOzfWUJmQgpbwNscAnEszR15h8sfOqVc7M1zJI57tVq1dsl5tvHquPM+Unh6
LmF/AwAA//8XuqnjTQIAAA==
`,
	},

	"/templates/helping-out.md": {
		local:   "templates/helping-out.md",
		size:    1168,
		modtime: 1490435396,
		compressed: `
H4sIAAAAAAAA/4yUwY7kRAyG73mKfzUHQJqZB+DGMIJFAi3SjoQ4OimnU3RVOSq7OhueHrnSPTuDOHB0
xf5tf7YzvCxcGVGhfOFKCRvtil0aJipYOK3IdI7lhD9/+Pn35yeMbMb1vrsEKd8YFrowTHAusoHKjkmC
B5j0+A/DgAf8UaP5Y5CpZS5GFqU84mVhjGIozEHx/PbjB3xmxtyqLVwxcpINs1Rk8YLLLJCC7T9l3yac
JHB/+Mhp9QdpBumaK8uaXAsh6iQ1dL+f4hdsC5nzgO2rKKgEnCrlTBVcq9Te/RxLwLfWAVJlZO9dZtjC
+TtXemq7G6AAXWniYbi7u3st7F2zw3DFGxWB51iicdohha+KR9uT5DXFiYyDY9PeTK/gvhfpjpVJpYAU
aocjzx7r7Gwh8xQRa+WZqwuOTS2OMUXbu4by1Kob4gCSyFnvjxyuQEhiXtM74q7pI+TQ01zBmmBuKe1o
FlP8m7t8K4Gr2q3amclaZe2Gb8Ja5RID6+MwvHx6/vQ9PsfsYqcWA/9r5HoQ/VGK1Ti213kPw22vonZ/
4+JjPslXTBuPiMW4zjT1A2jq4WuiWPDx5bdfsUVbMIqYWqX1YCOZoZlSwl90IZ1qXA3HBhy6lR/xy9z3
Y6NiziDTmUGYXquUghTPjDl+AWFsp3tQCKAbjGsqb9Z1dJGWgoMZaUw7ilicd2QXqGoY99uJEqJqY0iF
culHmD11ZlU6dXrXTXf5vgxfz3gjNXa7wmLuZzGRMqJ/ASUVbFJ7Hik9+PHA/z8OaxgefJgPR8BT2/v/
4d1pHA7/BAAA///lful7kAQAAA==
`,
	},

	"/templates/quickstart.md": {
		local:   "templates/quickstart.md",
		size:    187,
		modtime: 1490434533,
		compressed: `
H4sIAAAAAAAA/xzLsU4DMQwG4N1P8XMTZKjEK8DCwHgbQqfQsxKrJgmxA62q5tkR7N9HaxZDi4nxI6rI
rA2XOvA15HjSC4wdo8Ez46P6geiFO0MMsUA+/x7dvUV1OJ/9/f56NY8uRyzO5v/g0EpabjcsrzVVrOLK
WPnseFweiEJ4qrqHQBSeRzf55kC0baPs3FUK79tGNKd5lxN77nWkPCei7b8BAAD//yQZdPa7AAAA
`,
	},

	"/": {
		isDir: true,
		local: "",
	},

	"/templates": {
		isDir: true,
		local: "templates",
	},
}
