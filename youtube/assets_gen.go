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
		size:    9,
		modtime: 1499120954,
		compressed: `
H4sIAAAAAAAA/9JVCPF38VfQVQAEAAD//zTsWAgJAAAA
`,
	},

	"/assets/youtube.html": {
		local:   "assets/youtube.html",
		size:    5422,
		modtime: 1500655936,
		compressed: `
H4sIAAAAAAAA/8xY3Y7bthK+91PM4QlwbGAlnSRALxLbwGI3LRZFu2nSRdurghLHFlOaVEjKu4agdy9I
Uf6VZTvNFvGFVyvOzzffjGbGqiqGMy4RSFb8uVKlLVMkdT0YVJXFRSGobY5ypIxAXNeDMeNLyAQ1ZkK0
eiTTAQDA9t1MiUjMo5evwpk/z1+2xwWdY+TsoSbTPxqXUBZCUQYzRGbGSf5yS7OYfkTJDFBYoDF0jsAl
MG4ypRk85iiBgilTk2meIoMsp1KiCBadmsRHWHKGapwUAW3C+DJc/ieKIInXmCGKpoNwvscBFaitCSw0
alo9NgpfRsrWeUEdaP8dMZzRUtgtyU5pTyKX8z0597lmzIW9q78J+rjJVLFVhz0f7riY/pqj41ojY9zC
jKNgwA18Ko0FmyNIukBQM3+9kRtKBYlOXN5mWknrJLgdXQE+0UUh0LwBMqcLNOQKyKIUlmeqsKiJ5/YA
y0zpRYvaXUdL1JZnVBBYoM0Vm5BCGUuAZpYrOSHJgko6x6Sq4uvM8iX+UHLB4rvbuk7amj/0E4pvnE7f
a+XqB5DbHPW6wu5u4f4DlAa1CzseJ+kU7qVYgZItCYsrkMpCqmwej1Pd7eQ3V8MrVcJcgVWeu9aFe1au
gDeEUsY0GgMp1ZBTAzOlWwZhnCmG0xBMnKlF4nAl+ev89XutWOmZMOPEizlr0ptswbsk7sseg8td8v5n
OlwG1MnDjY3efUh/j16trr/Lrj/P2I/34uHxlwP33FdPY+iE0lE43ysNVK7gc4nGA4dPistQgUWhtAWD
eokaaKqWuO4BB6neehZ8Vc21KosjdeEVBE1RuCxMyMpGIfiIs01Xu1lXyjjx0j3WttxzWZT2pH+vZQoq
O9QiypiSpDM748QpnbDrbYFdFTghFp8s2aEmU9JqJQhwth+7bwETEhgIBNzd9vF42JfOOfpK6XJPwCZh
D+F5+AbS5Z/fZ8yVD7wzWw+ekm8pXwEzmd6GsR+Qnk6TQYGZ9bG3RrrZaYgI9m9af73Eu/XgyQbZ+6Jp
PjsTJpyZuu5hs4H4jwjNcsz+StXTSTovqKS1zUDNTyhdhO+WqFdKIplCuAMYbvXE2OO8L8S0tFbJAMiU
6YJviju1ElIrN8vSNWPjpNHoWBoSl+5p30K0/++zLWc3pdYobcfSar7awlZVL+auBOHNBA6Wni7pFsGB
Qk8NV5Wmco4QfyzTrvNttI8oBLivyCyOLVsHix2Xgks8vdY1sW62Obfp+e2uLBi1x5Y76G2eOWcMJXR1
CTfmllSUOCHBUZ+HSxufVyq6ulRkLLU8I24nrap4t2v/TBdY124JPbriQP/j9lxoD6A6xv59kF9jnsBz
zxQ4Nlc2D2i8xWI/0N7RApfxfMaIgTPHDFw6aqCq+Azivdt1DV4RGVQVSlbXF4wkODWWzqHnnPFkyixD
Y4hvJF/QuT5S95vl2Fi7CApzzVpfjIShQIfk1v/tx9I1ZnuYDHm7ZCo3712agdy+HTjzdc7+Oxs3fAs7
HbwYzkrp+RiOKm9lSTVwdudLdAIvhuS/uz9yRm/XYm6NPiLoN+zR24GXXcvFSg6b3Z9cwdox1fPgG5qf
2MONwpKK4SgWKOc2hyn8fwTVDmMBaUyt1UPCuKGpQEauwOoSA1T3qVEYPFN1RoXZ0fVX9WjQRNPqnRlL
K34qkk3MXxBLj/LRaOrRYJy0dbD3um+mlH8HFdf1IBTq3wEAAP//QZb4Yy4VAAA=
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
