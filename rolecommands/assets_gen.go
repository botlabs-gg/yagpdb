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
		size:    1405,
		modtime: 1505667139,
		compressed: `
H4sIAAAAAAAA/2yUT2/kNgzF7/4UL5tDW2A6QJpbLkX/Ib1ksdjksrdwbNrWVhZdUbLH376gNDOZBL0J
lkj++PjoZ/Y9SNUNgQ6eEcWz4keJ5YRWpolCpyBFGnkDRUZL3nMHCUijUxwk/QRX7kEH513aynlwC2OT
HNVq1MR9lAmEjnsXuIN3mnaWW+ZZlDskQecit8lvFu/CAMkJn54o0MD4akk+7Zvm22+PX/78HeRVMJKC
4CVBesicnAS1umgpICtb0siaomsTKHQYouQZeS6UhWvfNLe3t3gZGZ3re44cEibpWMHH2ZPBNg3u9vgs
gR/wYn3bPTphRZA0FtQ0ckQaKaAduf0HNJALmqocVlUR+d/sIneFxA1B7Fwgdu+hrUQvscZZ9jwXNd8P
Zt/glz2eXRh84eKtZJDgN4y0MO7qexfeIPYN7vd4yj652aK+ncoqV9InF9yUp0L4RMdylvMECRNPB44l
olR4lxpVyb+ONM2ekZUGfmieaSvNlfcE5bhwxOrSiHv01NaZWb2VQsLMYsFJcDBL1WM1qZVyEbKGcxzW
kUM153dxYY+XkZJCXQHgIHkYQd5j5VrfPCZw+tA0+Bl/RKbEpYP72uLV1/sPW2DzSKPoyTZN81lW8MJx
k8BFkTfKSdkvRbAT6I2NJ7KtCqGVbHhONbNWJcrElVMxplE/VMudx3NKfdmoSSJXs92dSxQzfKnqWUgf
mf2G73maceC0Ml9k06Z5EfTuaKzKmKMcPE9qMlloWxUgBF7xWDbmRMnV+a/VdK9lbG9tY468OMn6Qbok
xprONnm09Dcw/U4Fry17UexvWcvlylDxC5dWbBk+dmOlw68Xrbj+zYzn9WvduKtNKAyvpnVyYdghyFoN
1FL4ISHyJMv5V3jt7l3p9fyywk50/L8d2+FZrnOWEVCUbGKFzWZ3g/8CAAD//1L+/uN9BQAA
`,
	},

	"/assets/schema.sql": {
		local:   "assets/schema.sql",
		size:    1650,
		modtime: 1505851095,
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
		size:    16730,
		modtime: 1505667139,
		compressed: `
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
DrnyiYbP/wUAAP//6AKp9VpBAAA=
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
