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
		size:    6002,
		modtime: 1502115178,
		compressed: `
H4sIAAAAAAAA/+xYX4+cNhB/v08xdfvQSgWUqk8RS3W9S6NIyTVS0kp9igwewDpjI9vs5oT47pVt2Nu9
YxeaP1VSdR8Wg2d+4xnPP7vvGZZcIpCifSdUVXFZkWG4uOh7i00rqA1TNVJGIB6Gi5TxLRSCGrMhWu1I
dgEAcPi1UCISVfTkp3HOz9dPpumWVhg5PNQkexlEpkn95IC4zV5RSSuEBo1xT6EqA1Qy2FLNVWdA2Ro1
2JrLysRp0o6rSBjfjsNvogiSeL8WiKLsYpx/oBsVqK0ZtQtsWu0Cw4cpezDfUokC/H/EsKSdsAeUs9Te
OG4fjunc7zlK1FSAQWud6sdA99qfxs4Vu5sBTkulG9BK4Ia4IYEGba3YhrTKPFzxHPq9dZYoR4v9fIb8
EUuNxW2u3i+wAKSC5iiWqABSLtvOgr1r8QAdJG1wQ/4wqN1o9M1nkuYCGYG+5yXEV0qWvIrniYYBPBqy
vkfJhmF5KYEzYdy4J3QjLozBmOY6WQb5S3VQUBmEA/UoxkMg20Ma2HFbg60RdrXiBgrVNFSyeJ2Mww3J
TVRQIVRn4X4YcVkqkqV5dqMsPk2TPIMX5bgWqhGUBCpD5BrUW9T3C8qVddPjOzeAwaA/ulnp/rg+UkQI
yHHSELcogZdwpzqYDMk94J3qJmFBzZkgmVE1WeFHC0hL019TRNzw4nYxIuaJPj4i5Ij7CSNigvyoiPjy
ncQl8qjSqmsX3WTvKL8KWtwKbiwUNZUShYFSq+aoEk+ar8wbBgUWdlpU0wnLwycCnG1IPkmMJomT2+3X
guxqP+X5W1enptEK5dyv75u392U/4L1TreVKmnceiwC5lxNfFpZv8XnHBYunz0De+JU7HrL3/NdUG2QH
q50UGYZl6yTBFp/Ljc5N/TfLd2gdXwV/NVdU/slxd40C7aOUdY70AxLXpRBqBw02ucs1PrWkhWI4dbOT
oDTxX6FF3XBjnDOBVbDluAMWpE/xZr76JPTvbv6zLeo7JfHsts8TffCG4wj3he7h58oAR0eeWZ68s1bJ
cadMlzfckn0HaSXkVkamKwo0xo9F5R+5UMUtgewN3aIzMbwZTztpEhAzAPiElkgTVyizc2eph6/hlBjO
du6cOGemjzj5QSGQ6pLPBc3rLhe8OD4ZOyO7pnnsqh9reCioEyLSvKotyXxUSGUh/o1rY1/TCochpVBr
LDfkF1pa1Ju+j29wh8YOw373JN1Ggstb2G9j4x+t5g3VdyRzHDpNaDbGUkrXsk7CcyyVRif9d8GC9MyN
POrcUff86de6XjLSaFolDd/ONQ2ppzliOHXwtfpMqNg6e3GdJrY+T3OlkVpky4SXna2VXgEYuo4VgKGB
OU2YJqcU7PvvKni6Oe6OXlyfaHT6XlNZIcQvVXWqGVqwJcu+7XsvIU0sO0/Z9y6QqX3LG4R4tO+lXcca
BzMPA3y/f3Fif1hmdiscjX9DG1wj73zi9JcxKy5gjphCWRy9t+aMoSRj7rX43k4VkjMCWyo63JDRsATW
dPGHVXfK5d7c3pk2pOyECIXvcY4PEc6cL+i99OuReI1wegLyQdJIWp8cE+elw5C43JhMSmau1LvcsVBm
H9eC4/lTm3cuZnwKnKk7Psf8ozQWykSplEV9rkq4gINXd9BQ+X81WFe/V14eH98QH8p8dGketilcLF9M
XvB3AAAA//8J3aHNchcAAA==
`,
	},

	"/assets/log_view.html": {
		local:   "assets/log_view.html",
		size:    5244,
		modtime: 1501287538,
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
