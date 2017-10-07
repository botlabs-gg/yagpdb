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

	"/assets/help.md": {
		local:   "assets/help.md",
		size:    1951,
		modtime: 1506884622,
		compressed: `
H4sIAAAAAAAA/2xVTW/jRgy961e8bA5tAcdAmlsuxfYD6SWLxSaXvZmWKGk2o6E6M7Kif1+QIzmJsTfB
FvkeH9+jnti3oJRcF+joGVE8J/wq0Z5QyzBQaBIoIfe8gCKjJu+5gQTk3iUcJf8GZ/+Djs67vNhz506M
RaaYFKM0bqMMIDTcusANvEt5p71lHCVxgyxoXOQ6+0XrXeggU8anRwrUMb5pk0/7qvr++eHr33+CfBL0
lEDwkiEtZMxOQlJc1BQwJdamkVOOrs6g0KCLMo2YRmNpvPZVdX19jeee0bi25cghY5CGE/h19KRkqwq3
e3yRwPd41rn1fzTCCUFyb1RzzxG5p4C65/oF1JELKRc5FDUh8n+Ti9wYE9cF0WcjsftIWiFaiaVOu0+j
qflxMfsKv+/x5ELnjRcv1kGCX9DTiXFb3nfhjcS+wt0ej5PPbtSq7yts4sL00QU3TIMxfKRXe5Ztg4SB
hyNHqzCED61RlPznlYbRM6ZEHd9XT7TYcPY+IXE8ccTsco87tFSXnSneTCFjZNHiLDiqpcpjMalCuQiZ
w1aHuedQzPlDXNjjuaeckJwR4CBT14O8x8wFXz0mcOm+qnCDvyJTZpvgroz47te7ixToPnIvabVNVX2R
GXziuEhgU+SN5ZDYn0ywleiVrieyRoVQy6T0XEoTp6KEbTxxNmMq6/tiuW09a+tzogaJXMx2u0GYGb4W
9bSkjcx+wY9pGHHkPDOfZUtV9Sxo3atyTYwxytHzkFQmLa2LAoTAMx4sMStLLs4/FNMdbG1vY2OMfHIy
pQvpsijXvNnkQdtfQfVbAd9b9qzYvzLbnzMjiT+xjaJhuJxGocMfZ624XDPlc/hWEvcuCcbhoFpnF7od
gszFQDWFXzIiD3LaTuF7d+9s1u3NQnag159lbIcned/TVkBRJhUrLLq7K8uK3TQ8cpiq6nm9Rxg4TBjo
RfEzRknJrSkwzDUfq+iF53EBNY1KE/msiVhck4Zwb/vWiE8j6A1mV64ge8rbIbKLuoavacpVpm0s1dmy
7MJJXorE65Jx0HLjnnu+sYKbQAPfqO8PVfW5zRr73tW9tZid94qS6cUiHC2r2rHEYIxSc9LrrMocJW/X
M2EoH4R1uISR41AuSOLQXP6+bqbuKQT26wXQvO1A2TOlvEZ7Rd6BjKq0K1uXC1kzaWBViukSZuVZPi+W
EeVTPgrbB8j2krL2onH0S8nFhdnOy9n/HwAA//9ZdavvnwcAAA==
`,
	},

	"/assets/schema.sql": {
		local:   "assets/schema.sql",
		size:    1650,
		modtime: 1506884622,
		compressed: `
H4sIAAAAAAAA/6RT0Y6bMBB8xl/hRyLlD+6JI74KNSEVodKdqsryhT26FfZSsNWoX19BQ0JyTkJ6b5Zn
7N2dmX0Un5L0gbE4E1EueB49LgVPnni6zrl4Tjb5hjdUgSwbcnXLQxZgwV+xbKFBVfW09Otyyb9kySrK
Xvhn8TJnQemwKuQ/Jhp7oM1ZYJQGbmF3ctnAL4cNyK5Uu3/07fucBVga8t1rKsDzuXaVxboCqdXuKozG
A7doygqkcpakpbI709sbfyWqQBkPc2ibDLxjsdlR1SRdiOfLqspBrh1fp2MgHIDZBIe2pLUyxWSPtg0o
C4VUllvU0Fqla/tnPKWrixuMu30+zDZ6lIknkYk0FieihFjMOjkWYilywTfi5BNPwbszVFOLFuldEiY5
N6jt8W6A7nJPg3G9dRraVpXgEXX6jm1/KGOg8oP020BzEZL7+t7UW2V9yhvYWTke/Yq5B3Gu2vt/GYmj
TRwtBJuqt6S683/yxnxsxH13cxaApp/od8AZ3FIBsqf416fv3JOO8zb6RIXHON1Q6mLSx0qdu3wM/ZgV
nrG6Kixer1ZJ/sDY3wAAAP//TpIqiXIGAAA=
`,
	},

	"/assets/settings.html": {
		local:   "assets/settings.html",
		size:    16842,
		modtime: 1507386002,
		compressed: `
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
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
