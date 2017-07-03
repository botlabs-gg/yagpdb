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
		modtime: 1490713404,
		compressed: `
H4sIAAAAAAAA/9JVCPF38VfQVQAEAAD//zTsWAgJAAAA
`,
	},

	"/assets/youtube.html": {
		local:   "assets/youtube.html",
		size:    4602,
		modtime: 1490713344,
		compressed: `
H4sIAAAAAAAA/7xYUW/bNhB+z6+4EQO2AZG0tsAeCltAkHRDMWztmgbbngpKPFnsKFIlKTuGoP8+kKJi
x5aUeEuaB9cVeXfffTzed1bbMiy4RCB5/WmrGttkSLru7KxtLVa1oLZfKpEyAnHXnS0YX0MuqDFLotWG
pGcAAPtPcyUisYpevAxrfr18MSzXdIWR84eapH/3IaGphaIMCkRmFkn5Ys+yTq9RMgMUKjSGrhC4BMZN
rjSDTYkSKJgmM7nmGTLISyoliuDRmUncwJozVIukDmgTxtfh6zdRBEl8hxmiKD0L6wccUIHamsBCb6bV
pjf4b6TsrdfUgfafEcOCNsLu7Rzd7UnkcnWwz/1dMObSvm+/S3raZabYdsSfT3dRpx9LdFxrZIxbKDgK
BtzA58ZYsCWCpBWCKvz33b7vpYJEJ+7cCq2kdTu4/eEc8JZWtUDzGsiKVmjIOZCqEZbnqraoief2CEuh
dDWgdt+jNWrLcyoIVGhLxZakVsYSoLnlSi5JktdJ28YXueVr/KXhgsVvr7ouGer9OEYovEWWvtfK1Q4g
tyXqu+p6ewXvPkBjULuU40WSpfBOii0oORBQnYNUFjJly3iR6fEgf7r63aoGVgqs8rwNIdw9OQfek0kZ
02gMZFRDSQ0USg/swSJXDNOQTJyrKnG4kvJV+eq9VqzxLJhF4rc5b9K7HMC7AzzcOwWXu4P7zoyEDKiT
m0sbvfmQ/RW93F78lF98Kdiv78TN5o+j8NxXTu/oAaNJOD8rDVRu4UuDxgOHz4rLUH11rbQFg3qNGmim
1nh3/4+Oeu8e+IpaadXUE3XhDQTNULhTWJKtjULyEWe7jnZ5VymLxO+e8cZl3Viw2xqXxOKtJffA5Epa
rQQBzg6j+Qu3JCFmCPn2aqqijzvAUzPgimrHwU0osediwEcb5eDG4/iqLARQJL0K2hSgPJy8QYG59ckN
TsbT7zMN/i+HeJOO3V/bVh93ItabfFK1vy0ESHDiVG2/PQ6PgVx7bMgIkB/dYDCZRNJn8b84z0vM/8nU
7YOMz+d8r5rufAb2fkPpkn+zRr1VEkkK4QlgeDST40zwuRSzxlolAyDTZBXfFXhmJWRW7kT/grFF0luM
iF/iKiKdE/bD/z7bkHHZaI3Sjgxf5skGj7b9duUqEl4v4UjAx3YPCI4MhpIeNdNUrhDi6yYbW99Hu0Eh
wH1EpppqMEcDCpeCS5wfT/o8d1OJm1j8lNLUjNqpIQVmm2fJGUMJY03EiceaigaXJASai3BqX/RG9VgT
i4yllufEzVZtG9/v2r/TCrvODVOTUg3zV+250B5BdYx9fZBPITfw3JIDp8jO7sLuSU28R/J8HrOqA6cd
wyPUBx6pQHCqCkHb8gLig8ddB94QGbQtStZ1J6gVPKRYj6HnMcplmjxHY4jvMyc2tWvqxvIptTsJBnM9
XJ+EgqFAh+LK/zuPY0x5ZxgM53WKUPevFHqNHn74PvJNxcHriMM3F4VS/ud03L/Z8cj+DQAA//8B6dkq
+hEAAA==
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
