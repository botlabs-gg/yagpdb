package automod

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

	"/assets/automod.html": {
		local:   "assets/automod.html",
		size:    12706,
		modtime: 1502705242,
		compressed: `
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
`,
	},

	"/assets/help-page.md": {
		local:   "assets/help-page.md",
		size:    1609,
		modtime: 1506446420,
		compressed: `
H4sIAAAAAAAA/3xVTY8bNwy9+1fwlnTrOlkgpwA9pEAOQZtLk/a89IjjESyRsyJlr/99QWnGa/fLR+nx
kXzvjfypmmQJgCnJWeEiFUxAyaDOoJIJ0BFoFCBLoIIWhUHYoQWUyonKFlI8EuyROfIBzrTXaKTb68ko
NUFCPlQ8ECAHUAHh3WbzfSIIcRypEBuUmqgPMSCvU6CBTQRZMrF93GzgJ3h4+Jbk7PM8PHyEP5SKAhbq
W1DoK3CAjC/w9PIEmVTxQAqR4enyBEqDcNBO9RVVwbmjsNN9XcE2ocEgbBhZIUshP+LGWDk+V1rLFN7S
icpFuG83Ubm5C6LEb5yqsv2wzN+Eg8gnV+p/2i4IX0lsolVyhbc3N66PYqblEqKuUiztfot8bF1+J7US
B2v6uDUyOhSSAzr2F+SrhfclMw1xjMP19gYvJfwX2K82m82nlBZ/JzwRYLNXRhgkZ0/U3MRa/f3MuE/N
3S8j2BTVdxomGo4Utm1f5/JDasiw7fKcoxJEe6MQoraLzvdnlITdDnqZo2dlNCre4PvC7qSY3SMfK0eu
RrCn0X1HqC1kp1eaCVeqsIU4LpB2/LhWJxptpYi2wLVFxCZi2BfCo76ugweMvO13PtXpH1NbzATnmBLs
CQq5hs1/d9yLd/Ctx+E6jAkcFkRGviyjKZyjTVIN/OymzyjF67PX9Z67DfjvR3j88OH9tfxneISAF1fX
v6Fq9O7XOBzfeRqu0tq/SnvTrWU9KhR6rq5ka4+Qne7odHvkpu7KtCz8vmsUrWvB/vUBznO6dORcOerk
X+AOPp8c2WPug7ze9UdjDUqPVb77DP9mkPZ2alcDspworIZ20e8hZyxMYbfo9OXAnoUiPdstes35IKTA
YusSAjPJnKjZ1HfyonuaYUJmSnpP5UK1++DPXXsaKNHgL/ha0N5dj3UhmBOydrfN30xc/hGW2rFaLQT7
aj17HjQ4YaqumSdQahlot/krAAD//8KO+EdJBgAA
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
