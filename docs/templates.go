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
		modtime: 1499120953,
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
		size:    3245,
		modtime: 1499120953,
		compressed: `
H4sIAAAAAAAA/4xWW4vcxhJ+16+otR9iw1xw4sSQt9hLLpDEhjWEYAIuqUtSe9VdfbpLO6sT8t8P1d0z
o7F3D3mUuq5ffXX584ef3l2/BpugJesHcGgI0BuI6KFd4AUM8wIobgOOwC2Jpj6/2x3cMKBfYKQpgIUO
PQwkagpDiNRZFDK7pnk/UiT9neiOIk5wwCXBwnNWydoOb9V5DaYlEYqbLGLYfyUw4h2BMNx6PmSfHRtV
EM76V00DW/gjWtGfhrvZkRcUy34H70eClgU8kUlwvX68ghsi6OcoI0VoaeID9BzBsQbsewb2cHjQ7Nph
x4byj59pCvqDZwHONgNxmNQWGJs6jibL/Wjv4TCiKB4gS+CUIR0iOocRKEaOOfveegPPJAOIkcBp7tyD
jOSeq6XX86IfgAZSwI6a5unTp6fAZLSfZdw054Ib6q23QtMC7KmaLbl37MJkOy2gYpdyRjmMTY5UBSNh
Yg+YIEkRpF51FUAZMRPBQojUU1SD7ZzEtnaysmQbibo56gcrChPzbdoUH2oBYWLRmC5gV5taRzLZTUVX
GPp5mhaYxU72v4W/szcUkxyj7QlljpTyh9IhRL6zhtKuad6M1N3mh2G2hioRbK8l+CozQShS0hwz8/ND
4aTW4z8zJQ0u21ZAbrVVeFVyLcpTuLFOoy0+2B9Jj9CN6IecBn6WbsCBmgZe7OBHjre1PxoAgC28iYRC
+195sF51Byvj3NbH3/HODijZqCYWIn+iTrJB+KAg//VsFAnp+/2+KO46dvtP7DG9evlqv+AQTPu8WnsX
KRXkeg2jnUXYN/D1Dv7kGdLI86TlB2xLNRIRJHYkoyY42VsqVMz4XH3IubyrEf2OjuAXhwP99ezvv5VK
toMnhrttBmr7Yqs+d8EPT/7553kD3+xq5oDg6QBtRN+N0Ed2OUJDd/XfF8FX0RI+5Ka6+vD64qd9PJCv
t0V/W0TPEZVqTLZy6IsA3vqOjlRi/5nMBrqT5mV8OKD1pduWkEcIglewlHwH9B5hsHf5M146vPpQEaoG
/09S3xyT6rLGOamXO/ihF6qtvFyUmX1xeoZ/80D8VV4ndsIFPhagv1/HW/LRQnxs4Nsd/IaZKVQ7ItV8
fulXOTuVWfdMmObB+m0K1NnednkhbE7rJU/QMo48fKyy6naPKZGkvYpvtS12znz8Vw4H8nmNXToyfHZj
uEt7IRcmFEr7+/v7+5X1t4EKC3o7ZTBLA5bJGk5sJWPliOWpsOfKvdyqwIqMDxf4C7EVZzPcuRwV7xKC
LrycMTtnBRylhANtwFDqom21pQ9HUhhrHgru221R3lblx+N7SHIV4rl5azTH2fPdii1hniaIlOdw1fuJ
oUVtqzL+IrNAz5OhCM8GLgu6B5ym0o42UiccLaWj45vT+Ko8xhAI4wYOVkbAY2E83Ys6saIkV/Enb9gF
3dW5mqvInjyA03fbivzjAF2IrJC5IYEW06nrhOGa7h7w8WqrYrXPH/fzhdjK13WpPF3W/UzXOm4u6/Bq
B3+glbyoI6XAPlEZ045K2zg7jJJvRl0WucLkJZXKZZxD5MCJzJmgrhSdUwF45EBl91uBg50mnU+O4kDm
qinH0Bv2Em07n061fJHmG8CmTHYhr2078Pm4OVBb1n6PXb5d56TqYdKZ/PP7334t8bXMkiRiKBeNZpGc
suoT3qFiFqTsmWo30m41XDJzjuPlFCX7sjF7e595NmwAjQE8njDVVZ3CR3qGyC220wKexfaLXiC9jUn0
hj8dGjalmYAjJPL5fnbqunbe6mJR86e5Xy7wAyapw0Ksy+uoU/ZZfQGcEsOBY/aTdxxKOXv+zU3cNNv3
b6/fbovC63kpt+v6qi0C/wsAAP//WuEmt60MAAA=
`,
	},

	"/templates/quickstart.md": {
		local:   "templates/quickstart.md",
		size:    591,
		modtime: 1499120953,
		compressed: `
H4sIAAAAAAAA/3xSPW/bMBDd+SsevHhxZPRr8ZYiQJaiLZB2yEiRV4kIfceSR6vMry8oKw26ZBPwPvWO
5sccCpKdCEuIETPFhCYVv2twT7GhkKIm6EwYRQdjHkhrOhnzbsC9QAWzaiqn47HZKflx+NOejzDvB3zj
VVVTonyTwzTrjZPMlA9wMbgnCGP3RabA+FkCT7gLxUn2O/NhwEM4p9gQV7iucJOa4a8cuEyeWION5YCx
KrzwXrFIzu3QqftMPWDj7wsWGktQAstywELwAhbFRArLDfLrGvCfcfeJESMhkw+ZnJKHsKMVuRBs1Vly
eCaPx9v773ef+yCXQMv664XyhXJ5rTOYjwNuXzSbpC/WZU6YyWn/XJtY56SyDubTm1tW9pSxKxS72G6h
uwPKdcINeO2znnexrC9H7cXwVZROeJQKZxnCscF6/48S1nkw2wvhbLm/l80tUT6HUoJwGczfAAAA///i
AytOTwIAAA==
`,
	},

	"/templates/templates.md": {
		local:   "templates/templates.md",
		size:    2477,
		modtime: 1499739160,
		compressed: `
H4sIAAAAAAAA/5RW33PbNgx+91+BSx/abok7u2+9XW9unPS8H20vyXa3tzAiJKGhSJUE7Xhx/vcdSMmW
7ey2vcgy8eHDRxAAdVMjVA4Ym9YoRkBbkUWgADGghtJ5+HP28cv8w8sARQzsGihc0yirAyirgSwslScX
Aziu0UNrVIEBVsR179BgCKrCMB7d1BSgVRWCoiYAO6jRtLB2Ud4rZOAaoXGBwUUGV26F5WgHCsaj0ZfI
8Ph48vh48vQ0Ho/T78nTEyjvotWJzqoGAxi6R+Ba8Tu43Tr8HtCnh2B639vR6MULmC0VGXVncJcbrVi9
E+MLEJ/RaAOXhEbDBuYYCk8tk7OwGW3g7OwMuudoA7f7gW5hA5L3GNCnPKfVIXIx38eQztYff+rt7wVw
8aCa1qDkqfComGwFChq0SYdkr2MYcs9JpDZkFTu/H0YPTbCBndNsqVgdoFVa22rLwA+OE8pHBBoIoAAK
7hwLa8rgx0hGv7lGv/x/mUx+XYI+R24jhxSlknVYzPdgn7psHwMHGe8YC2efh1LRSemhV1jRP4F9su3B
Z+X9ea2sRXOsenb5CxTZeKj988qiP/ZwsnyI/dmRRT3jIXhVowWVs1+SDwxfE2og9kDlDTXoIh9GZGow
syWuCjlA45Yozc/uaBurmgyCdQyqYFruZ/k3bO7Qn7toj8LYKCap2lxeTtQf6/wDPZVUKCmQX3GJ5pDH
47dIHjUsB0gwAk3j7PntXzR3qC+sNLweMlIJIlmmnZfpmADgvGzwFNhHfFMqE2SXqazzBv97PX+i4r5v
K0vFfarLLJMCNIktI6+cwSDQGRgKaTh6ZxBIvwxpsOXZmT1qFXpFl9EWEj2IqO7933Up/RXT8UnEK+To
rXSwV1a7BrbWDNZUMNzjegJLZSJO5H2a36eAXAjHuQwoGeMgaHJW+TW8kjJplF3LmUOhAgZYI7/OtGTz
ThPRUAd30yUZZLT0yOTGxAYhsCdbDb3ySr6ZJFW5KQwy57JDVdSwcl5DoVpiZegv1H02NDzAesj2AN/D
OlsDfoPAyjMEdu3+Zi2uQHmv1hKBLGOFPpxmuKgpvWs6Z7ng0Oo0xDNVR1/HsjSYdijkfnsanSWVeuhG
/i4P0u7X7GXfE3lM5fFWGGRcBAi4RK9Ml5eQu3lyKkdRxtwr+IBFTDq3lz7ZwZ2c4khNLCzDK5F8Kr2x
S8brZ8unywPcIa8QLfyQ9n7gfpZTQCVMQfkqQOvdkjTqMfSlfW2pbZHD6Du5Gvdv9cW8v8/f3267eXc5
doMrTbRtRaR/i/kpaIfBvkwfIW2Uk4HFXIL0IUjOErppNk6dCSeT6dvJ9GT7BZK+dVZkDDhr1hBqt+pv
xEGTpoklbZxEkB7QJCa0evdd8ncAAAD///ku1JKtCQAA
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
