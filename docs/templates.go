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
		size:    639,
		modtime: 1490690428,
		compressed: `
H4sIAAAAAAAA/1xQz46eIBA/16eY0jOSr2eXy+656Rs0UxmVLA4G0U1DfPcGPzTkOzEZ5vc3JUODZQJh
fL9Kxl0cR9M5qxsAgA5hCjS8iR9CdxZ6h+v6JgaEAaXlwed3+BK6U1bDh++3mThitJ6hWxfkCoEh+PMy
73WnsChs7jpi3IFxlyv1no10tJMTuvmWz1IKyCNB++H73zjSehwn/KS43N4LrCils/wpSgyVU6qU2l84
03EIfY+3oZNAZcoiTGyKWPddSlDtq0eQsmRRm9PNE3zhmpeGFxwpV5xSpHlxGAlEv/yZCI2ANndv7H7Z
Pxt7Ulfb3jvpRvn4KSrH0+P6zgoy81E4871vIRDHUtwdd3pU6Ip9xvBp/BfLv978qxRSyt2/e47EsW5f
GbuXszKX5zXj4H2k8ExZ6vkfAAD//68Y2Bt/AgAA
`,
	},

	"/templates/helping-out.md": {
		local:   "templates/helping-out.md",
		size:    1173,
		modtime: 1490711186,
		compressed: `
H4sIAAAAAAAA/4yTwW4kRQyG7/0U/yoHQEryANwIESwSaJE2EuLo7nJPF1NVbpVd09s8PXL1TDZEHDi6
yv5tf7aHl4UrIyqUL1wpYaNdsUvDRAULpxWZzrGc8OcPP//+/ISRzbjed5cg5RvDQheGCc5FNlDZMUnw
AJMe/2EY8IA/ajR/DDK1zMXIopRHvCyMUQyFOSie335+wGdmzK3awhUjJ9kwS0UWL7jMAinY/lP2bcJJ
AveHj5xWf5BmkK65sqzJtRCiTlJD9/spfsG2kDkP2L6KgkrAqVLOVMG1Su3dz7EEfGsdIFVG9t5lhi2c
v3Olp7a7AQrQlSYehru7u9fCbInvOh6GK+OoCDzHEo3TDil8lT16nySvKU5kHJyd9o56Gfe9UnesTCoF
pFA7HHn2WAdoC5mniFgrz1xdcGxqcYwp2t41lKdW3RCnkETOen/kcAVCEvOa/oXdNX2OHHqaK10TzC2l
Hc1iin9zl28lcFW7VTszWaus3fB1WKtcYmB9HIaXT8+fvsfnmF3s1GLgd3PXA+uPUqzGsb0OfRhuyxW1
+xsXn/VJvmLaeEQsxnWmqV9BUw9fE8WCjy+//Yot2oJRxNQqrQcbyQzNlBL+ogvpVONqONbg0K38iF/m
viQbFXMGmc4MwvRapRSkeGbM8QsIYzvdg0IA3WBcU3mzrqOLtBQczEhj2lHE4rwju0BVw7jf7pQQVRtD
KpRLv8TsqTOr0qnTu667y/dl+HrLG6mx2xUWc7+NiZQR/QeUVLBJ7Xmk9ODHA///uK5hePBhPhwBT20/
ruDtfRwO/wQAAP//qmGjfpUEAAA=
`,
	},

	"/templates/quickstart.md": {
		local:   "templates/quickstart.md",
		size:    187,
		modtime: 1490690223,
		compressed: `
H4sIAAAAAAAA/xzLsU4DMQwG4N1P8XMTZKjEK8DCwHgbQqfQsxKrJgmxA62q5tkR7N9HaxZDi4nxI6rI
rA2XOvA15HjSC4wdo8Ez46P6geiFO0MMsUA+/x7dvUV1OJ/9/f56NY8uRyzO5v/g0EpabjcsrzVVrOLK
WPnseFweiEJ4qrqHQBSeRzf55kC0baPs3FUK79tGNKd5lxN77nWkPCei7b8BAAD//yQZdPa7AAAA
`,
	},

	"/templates/templates.md": {
		local:   "templates/templates.md",
		size:    284,
		modtime: 1490711189,
		compressed: `
H4sIAAAAAAAA/1TPsU4DMQzG8f2e4ttopZbszEhMqEs3xGAlvsT0Ekdnh7Zvj66IgfEv/ST7e1dzpHuj
KhHON4eNWECGL5WGymaU2Q6Iw1wrotZKLRmopV8qhsyNV3JOGCYtw7n2hZwNu6orwzpHmSXSstzx8XZ6
ssep8OfQKV4o8+euuHd7CSHrQi0/65pDv+TwT4f9fprORQydMoOkGlzBt76QNBS9bpnZ4YVRt4E6HDpv
XQ+Pz6+FHPNo0UWbgVZGX/VbEqdpOuJ8ej3h+BMAAP///0PdJBwBAAA=
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
