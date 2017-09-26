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
		modtime: 1506449968,
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
		modtime: 1506449968,
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
		size:    16818,
		modtime: 1506457889,
		compressed: `
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
ApvDs3yi/vN/AQAA///FkgtAskEAAA==
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
