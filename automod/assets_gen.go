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
		size:    12708,
		modtime: 1517699040,
		compressed: `
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
`,
	},

	"/assets/help-page.md": {
		local:   "assets/help-page.md",
		size:    1609,
		modtime: 1506449968,
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
