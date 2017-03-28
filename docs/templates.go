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
		size:    3307,
		modtime: 1490718542,
		compressed: `
H4sIAAAAAAAA/4xXW4vcxhJ+16+oHT/EC3PBiRND3mIvuYATG7wQwhJwSSpJ7VV39eku7YxO8H8/VHfP
jMbePeRx1HX96qvL/PXTL+9vXoOJUJNxPVhsCdC1ENBBPcML6KcZUOwaLIGdI41dejdb+MCAboaBRg8G
GnTQk6gp9D5QY1Co3VbV7UCB9HOkBwo4wh7nCDNPSSVpW7xX5yWYmkQorJNIy+4bgQEfCITh3vE++Wy4
VQXhpH9VVbCBP4MR/dhyM1lygmLYbeF2IKhZwBG1EW6Wj1fwgQi6KchAAWoaeQ8dB7CsAbuOgR3sHzW7
dNhwS+nDrzR6/cCTACebntiPagtaExsObZL72RxgP6AoHiCz55gg7QNaiwEoBA4p+864Fp5LAhADgdXc
uQMZyF6rpdfTrD8AW4geG6qqZ8+enQKTwXyRcVWdC95SZ5wRGmdgR8Vszr1h60fTaAEVu5gySmGsU6Qq
GAgjO8AIUbIgdaqrAMqAiQgGfKCOghqspyimNqOROdmI1ExBf7CiMDLfx3X2oRYQRhaN6QJ2tal1pDa5
KegKQzeN4wyTmNH8N/N3ci2FKMdoO0KZAsX0Q+ngAz+YluK2qt4M1Nynh34yLRUimE5L8E1iglCgqDkm
5qeHzEmtx38mihpcsq2A3Gur8KLkWpRn8MFYjTb7YHckPUIzoOtTGvhFuh57qip4sYWfOdyX/qgAADbw
JhAK7d5yb5zq9kaGqS6Pf+CD6VGSUU3MB/5EjSSDcKcg//18EPHxx90uK24btrtP7DC+evlqN2Pv2/q6
WHsfKGbkOg2jnkTYVfDtFv7iCeLA06jlB6xzNSIRRLYkgyY4mnvKVEz4XN2lXN6XiP5AS/CbxZ7+fv7P
P0ol08Cq5WaTgNq82KjPrXf96vPn6wq+25bMAcHRHuqArhmgC2xThC09lG9fBV9Ec/iQmurq7vXFR/N0
IN9usv4mi54jytUYTeHQVwG8cw0dqcTuC5k1NCfNy/iwR+Nyt80+jRAEp2Ap+fboHEJvHtLPcOnw6q4g
VAz+n6S+OybVJI1zUi+38FMnVFp5vigzu+z0DP/6kfiLvE7siDN8zED/uIw356OF+FjB91v4HRNTqHRE
LPn81i1ytiqz7Bk/Tr1xm+ipMZ1p0kJYn9ZLmqB5HDn4WGTV7Q5jJIk7Fd9oW2xt+/FfOezJpTV26ajl
s5uWm7gTsn5Eobg7HA6HhfV3njILOjMmMHMD5snqT2yl1sgRy1Nhz5V7uVGBBRkfL/BXYgvOJrhTOQre
OQRdeCljttYIWIoRe1pDS7EJptaW3h9J0Zr2seC+32TlTVF+Or7HJBchnpu3RHOcPT8s2OKncYRAaQ4X
vV8YatS2yuMvMAt0PLYU4HnPeUF3gOOY29EEaoSDoXh0/OE0vgqP0XvCsIa9kQHwWBhHB1EnRpTkKr56
w9brrk7VXES2qqrqaJwEaoynjhGGG3p4BMdXGxUrPfo0iF+JLRC8yVWjy5qdqVZGxSWGr7bwJxpJSzZQ
9Owi5RFrKVPemn6QdO/poE/VIScxo54w8oE9R2rP5LK5YBwzOAN7ynvbCOzNOOpssRR6aq+qagnDD5ti
42kILkRWnz/DCnF1XV3d4SggdJClmujCtme2weot9wy3RkaCWy3pi9V1PqXesJNg6ul06KV7Nl0QJqZW
EXLa9D2fT6M91flo6LBJl+8UVd2POtF/vf39bUaoZpYoAX2+hxTHaJWTn/ABtWpe8pYqdgNtF6Mp8e44
nE5Rssv7tjOHxNJ+Ddi2gMcDqLgqM/xIbh+4xnqcwbGYbtb7pTMhiv4DOJ0pJsaJgANEcun6tuq69O3i
3lHzp62R7/c9RimjRoxNy6xR/ht9ARwjw55D8pM2JEo+mv7NRV1Vm9t3N+82WeH1NOfLd3kTZ4H/BQAA
//9mn6Cw6wwAAA==
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
