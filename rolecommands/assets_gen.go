package rolecommands

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

	"/assets/migrations/1502731768_initial.down.sql": {
		local:   "assets/migrations/1502731768_initial.down.sql",
		size:    68,
		modtime: 1502779925,
		compressed: `
H4sIAAAAAAAA/3Jydff0s+bicgnyD1AIcXTycVUoys9JjU/Oz81NzEspxiKVXpRfWgCScPb39fUMseYC
BAAA///vBqwzRAAAAA==
`,
	},

	"/assets/migrations/1502731768_initial.up.sql": {
		local:   "assets/migrations/1502731768_initial.up.sql",
		size:    750,
		modtime: 1502782847,
		compressed: `
H4sIAAAAAAAA/7SRzWrDMBCEz9ZT7DEBv4FPjqsUUVsujgsJpQilUsSCflL/gB+/JHVCTE1CD72Z3Zmx
vp0VfWY8ISSraFpTqNNVToGtgZc10C3b1BtogtXCNKE/trAgESpodYPSnjX8Lc/htWJFWu3ghe5iEpke
rRKoYI8GfXeVxSTy0mno9DAZNvqrx0aL03/a0fT+EZMIjQ9zcxeUngl3ve3waLVwcri7Rj+zbtEbq4Xs
uyC6YE7f4XCAfQhWSz+jvDw7eP1LRZYJeXzTz+Cc9Oqfrnpt7cZU0TWtKM/opNUFquXomEn/Wz1n8hGc
8Se6vQcuDKoBSj6dLi6oy+Rh0g/ANGeEukkhWVkUrE7IdwAAAP//Ct+SVu4CAAA=
`,
	},

	"/assets/migrations/lock.json": {
		local:   "assets/migrations/lock.json",
		size:    3070,
		modtime: 1502731768,
		compressed: `
H4sIAAAAAAAA/+yVz4ryMBTF932KkLVP4PZbfiDD4E4kRHsbAje5Tv6AIn33oZ220wxGmEU1A27KJYdD
fgmnJ9eKMb6VBwTP12xXMcbYtf8yxjfSAF8z7ghBHMkYaWvPV6P8jzAa++2bexO/ridTv769nPp1D05L
TLU3p410l/9w4WsWXIREfYcGHNhjZ7cRMRE3FDYRcfBNSru6T6eixlrkGA9aaRvuMDYS/fKQVhrwtwkD
nMNu/3TCPiTKUTwtdZfzbcfUpjunNzRFdMjgTGozJ+4BfnXkolPj4CNqB6IDzaTni7SA/GhlqRTUYdpX
M/Dbrfgjdq9OfHAn5ivx6Xivn28RVEN12aVrIgZ9QhBGnv8IqLZFg3ptFYKQMZAIpLqZmiaDTIQgbSnM
YwmQzYX2Ubzpm1Z1U/sZAAD///1wl5X+CwAA
`,
	},

	"/assets/settings.html": {
		local:   "assets/settings.html",
		size:    8122,
		modtime: 1502779925,
		compressed: `
H4sIAAAAAAAA/9xZX4/auBZ/z6c417dSQZ2Q0lv14RYijVqptw/TuZrt22pVmfiQWHXsrG3IzCK++yqO
gUDDEJg/ZTsPQxKff/6d4+Ofk8WC4ZRLBJIU37QSmKg8p5IZslwGwWJhMS8EtfV4hpQRGCyXwYjxOSSC
GjMmWpUkDgAAmk8TJUKRhsM3fsyNZ8PVcEFTDCt7qEl8owTCB+94FGXDhkoRXwqhSgMFqkIgWAXUGJ5K
sBlyDaqUUMVtBvA/aoBCyRnCnGqO9g7UFAxay2Vq4E7NIKESbIn0+2AUFT7oiPG5v/xXGEI0WIcOYRgH
fnwHCipQW+PBqNW0KmuFrti8a0JT2bhkDCSWbj7gE+FMrqUaRgoqUYD7Hxaa51TfNey1SjvAuUx35Kq/
NtfbxjYw7bc/UWw3CCc4VTpfSVbXYaY0/0tJSwUBmliu5JhEOZU0xWixGFwmls/x04wLNvj8cbmMmpUZ
SSy/JTlrcbQblfOVajUr9gi3Zcbk21XbqiPoBAVMlR4TiWVYxRf6AENJcyTxF5rjKHJyB2w1/HNZzOzB
iNeapqCyRTWkjClJYhcWjKJKrIM1ZwHsXYFjYvHWki0YEyWtVoIAZ/vmDNX/Mfni5n//nH+spo7D9w09
QubfPSTx3t+n6meV+tFEH7BoUGBiPXZO9xjgO9eKKqplFn9REkeRvzmQozqyk9IETwBvdVNvFt3WlQe2
gi3JqJQo9gBbQ3/jzHcGklL6DDiec7lr/HPGNbqHhsQ39a2pt+MT698buXEmV8HmM2G5F9vOnBspBHqZ
QuCeZbIT65llGZ4gOzyVajPhz+7uQampTTx6ZrbjPLPE3Dc0mVmrpN8vzWyS882OObESJlZuiNklY6Oo
1mjhR1GFXXwf32qy1H2E9d2aK55MNl0L+YmUsw7geYjnUV21K0sVWyz1GTpyTfiOpZxHUr2Gk24c7+l7
XR1SrhiS+EoxfEK+1XAFSlZUIsUxqZx+cNesZzNuLl5Wsi/73XsYzKmY4Zi8Jkewshb9IYl/4zIVp1t4
Q+KrqkuLoquRX4nU1Pk9czbTGuSZ7ZbwmPk4Y/7SFuCZpeL4JbYzO+M6SqgKe9/kTsl2UyXD5PtE3XaB
r8O2tpZtbm9rH74M6lbpV+a1RBKDv4FhzUW49DyIWqBCgOU5GujRqUUNXHLLqfAvIXOUtn849k578smv
ROCfnIjLmVVfVZoKvJ5OXS5yNUfAW24sl2mdkTJD6RGvnlGpbIZ6O1uDn52GBy8512ieYsW1UycuV73r
ikuez3KQs3yCGtTUIdu9zZ7EJjf+fT1c+T57xeU5MEt6u4aH3rbAc8Q2dBo+6wB28aH3LdNf6zDrh0/8
3FQZdAQfRN1NDJTcZqvDptk+6GZv6ze3ZhRlb5/jAOy8/Rcq7v94J94i/nr98Xr9detwBp5ljsM3//mJ
U1xVkf/Z+lwHEAQjk2he2DiYzqQ75kPzcMe0Kpgq5QVw9n+NU37bXzizc6qBM7csrwtrYAwveuTfBF6t
BeEVkB96e/99Q7veBA+qN9lY/33gDPDpOrSBO8fBeDwG8pr48JzMJrwBZexDBW+PZJwxlKtAarlNIHsF
lygM7nM7fLBb7Xb+XdmV6CaCdjft2kdOsAY2UdIogQOh0h4plZoqpcgFbM+6/x6CZRC86K1qpufn3yyd
KqE7bxD6v7/+4wJcn6/cLvvBKPLltynl3a/NU6Us6vprc7BYoGTL5d8BAAD//020WIu6HwAA
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

	"/assets/migrations": {
		isDir: true,
		local: "assets/migrations",
	},
}
