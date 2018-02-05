package youtube

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
		size:    547,
		modtime: 1515026406,
		compressed: `
H4sIAAAAAAAA/2yRvY7TQBSF+3mKs0oDCJyCn2I7lAW0aIXAIkUUUUw81/ZVxvda8+Pgt1/FcYpEbo/m
fufTmdVqhd8aU4Ro4porm1glgmtYCJ0wsCNFHbRDagkNDySoWitCHhyRe6/WkSuM+atoKN2BpkuLUXPK
B7pevkfkrvcjWPqcQJxaClPB8xM0YFu+gGUKavVeTywNaibv4qMx77CbcZtZ5PnJPOzvOsDu35s2pf5x
veaCuyaHotJu3dpSfn5yRS/N2yXWtnxZgOXgF2mbj99+/ZEfM22nGZW9DNTQ5H8FpNamKbjdxwZCrzGR
Q1IMbKc3Lmjv9CQfOpJszMP+Giw6fP9Sbuznr3cO1kdFR3LuAQ0URhXCYUTluTqeB73oUXU86H9kcfMX
3JQX5jUAAP//vwYksiMCAAA=
`,
	},

	"/assets/youtube.html": {
		local:   "assets/youtube.html",
		size:    5468,
		modtime: 1502705244,
		compressed: `
H4sIAAAAAAAA/8xYTY/bNhO++1fMyzdAbWAlNQnQQ2IbWOymxaJoN026aHsqKHFsMaVIhaS8awj67wUp
+luW7TRpswdHEefjmWdGMyPVNcMZlwgkK/9cqspWKZKmGQzq2mJRCmrboxwpIxA3zWDM+AIyQY2ZEK0e
yXQAALB9N1MiEvPo+Ytw5s/z56vjks4xcvZQk+kfrUuoSqEogxkiM+Mkf76lWU7fo2QGKBRoDJ0jcAmM
m0xpBo85SqBgqtRkmqfIIMuplCiCRacm8REWnKEaJ2VAmzC+CJf/iyJI4jVmiKLpIJzvcUAFamsCC62a
Vo+twqeRsnVeUgfa/0YMZ7QSdkuyU9qTyOV8T879XTPmwt7V3wR93GSq2LLDng93XE5/zdFxrZExbmHG
UTDgBj5UxoLNESQtENTMX2/khlJBohOXt5lW0joJbkdXgE+0KAWaV0DmtEBDroAUlbA8U6VFTTy3B1hm
Shcr1O46WqC2PKOCQIE2V2xCSmUsAZpZruSEJAWVdI5JXcfXmeUL/KHigsV3t02TrGr+0E8ovnE6fauV
qx9AbnPU6wq7u4X7d1AZ1C7seJykU7iXYglKrkgorkAqC6myeTxOdbeT31wNL1UFcwVWee5WLtyzcgW8
JZQyptEYSKmGnBqYKb1iEMaZYjgNwcSZKhKHK8lf5i/fasUqz4QZJ17MWZPe5Aq8S+K+7DG43CXvG9Ph
MqBOHm5s9OZd+nv0Ynn9XXb9ccZ+vBcPj78cuOe+elpDJ5SOwvleaaByCR8rNB44fFBchgosS6UtGNQL
1EBTtcB1DzhI9daz4KtqrlVVHqkLryBoisJlYUKWNgrBR5xtutrNulLGiZfusbblnsuysif9ey1TUtmh
FlHGlCSd2RknTumEXW8L7LLECbH4ZMkONZmSVitBgLP92H0LmJDAQCDg7raPx8O+dM7RZ0qXewI2CXsI
z8NXkC7//H7BXPnAO7P14Cn5mvIVMJPpbRj7AenpNBkUmFkf+8pINzstEcH+zUqUUUsjjR8rrrFEXZjI
oGT9+XBbw5MNJu7LtiftDJ5wZpqmh+QW+T/iOcsx+ytVTydZvqDA1jYDYz+hdBG+WaBeKolkCuEOYLjV
E2OP874Q08paJQMgU6UF39R8aiWkVm52qGvGxkmr0bFLJK4Kpn170v5/v9jOdlNpjdJ27LLms+1xdf1s
7koQXk3gYBfqkl4hOFDoqeG61lTOEeL3Vdp1vo32EYUA9xOZ4tgOdrDvcSm4xNPbXhvrZslzC6Bf+qqS
UXts54PenppzxlBCV/Nw029BRYUTEhz1ebi0H3qlsqt5RcZSyzPiVtW6jneb+c+0wKZxu+nRzQf6H7cv
hfYAqmPs3wf5OcYM/EejBo6Nm81zG2+R24+/d+LAZfSfMXngzOkDl04gqGs+g3jvdtOAV0QGdY2SNc0F
kwpOTatz6Dlnapkqy9AY4vvLJzS099S94RybdhdBYa6H64uRMBTokNz6f/uxdE3fHiZD3i4Z1u1XmnZO
r74lnPnxZ/8Lj5vJpZ0Ong1nlfR8DEe1t7KgGji78yU6gWdD8v/dV6LR67WYW7qPCPp9fPR64GXXcrGS
w/ZNgVzB2jHV8+Ab2hfy4UZhQcVwFAuUc5vDFL4dQb3DWEAaU2v1kDBuaCqQkSuwusIA1f01KAyeqTqj
wuzo+qtmNGijWemdGctK/FQkm5g/IZYe5aPRNKPBOFnVwd7HwZlS/otV3DSDUKh/BwAA//+H3KheXBUA
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
