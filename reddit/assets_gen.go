package reddit

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

	"/assets/settings.html": {
		local:   "assets/settings.html",
		size:    3593,
		modtime: 1499552705,
		compressed: `
H4sIAAAAAAAA/7RXTY/bNhC9+1dMiR5aYGVlcyjQhSxgsUGLXHqocw8ociRxlyIFftgxBP/3gpRsa72W
YqcbH2RJHM68mcd5pLqOYykUAmHtV4OcC0f2+8Wi6xw2raSuH6mRcgLL/X6RcbEBJqm1K2L0luQLAIDx
W6ZlIqvk/uMwFsfr+8NwSytMgj80JP83RszS+n5k2+ZfaoQeDLTSV0LBVkgJrbYuXiyURjdgfdFbWRAK
dtob4MIybThYNBs0Wdq+8psV+T/aCYYPWVrkkDHNMYc/XQ3PXu6yND4/wOcQXHBwJxglIrfgNAjFpOcI
pBaco+qBpCalUpIRoDsQZUAEKsaLrgrtoBHWClUBVbshk1YitQgSHTQIL0pvl0fYWcrFZrj9JUkgXR6L
C0mSL4bxM7KoROPsQFc/zehtP+HH2BuNt1ShhHhNOJbUSzeyvGgd2RaqOrMLv0fOQeE2lve1k1Pm034L
zXcXnA4L6EgGlAIlB2Hh2VsXqVC0QdBlvD/Z/aZ0oDIsptJo5YKFcL/fAX6jTSvRPgCpaIOW3AFpvHSC
6dahIW8hlNo0B7DhPhFKCoUEGnS15isSyCdAmRNarUjK2rTrlo/MiQ3+7YXky8+f9vt06Mi3OZ4XJMao
jPbthHGcIGmBEkptVuSYNcnXh9ssjQYzDoRqvQO3a3FFHH5z5FV8ppUzWhIQfBwgVvvVi1ZShrWWHM2K
1HrLtbANSjmV6NvF8C41YDVVCiXJ11Et4Kl//n4ZLEpkLqZ58HG5EH3mxziTHsOv65ovpzbup3zVbVgg
lgAZwIW+Hi+Tw2sg6wgKOQHyIWj4JPq0h397rQvvnFYD/dYXjTgtgMIpKJw6acIj51naz7jQommoUz7X
8uePP02DnrwxqNxIvN9Nibru1yqwBA8reNPcl6zZgc3zCQeaL04zVFUIy34zfdKqFNUFuzHqLUoJ4ZLY
ZqrpfkjC+nyPyhVELQqZbzl1ONeXk9Jy2GgvNJbgBDZUelyRIdBchFulAt5DMudz+x+yOc587Yv51Keb
+l0qcxS4axUUfpqKQt9IdhDD2EgDqhlNPM28UoNPnTrS3SsDzQswXMFXONT95dlLOEnWrpGgldwBlVJv
LVTogCren5ap5V7oZ2r56aq5AFcLG05EHz/c/xE4hNKzFwuWvmA8KE6GvmYPsJ4xtJbEnr1NH9Z0g9Pb
xk0oeFBFcwsIjhIDiE/xfx7GpR1shrquQ8XP1sV3Nrz+5N7vdQdKrvwgODv1n38glFqHU2v8QFgMyP4L
AAD//zuFAJ4JDgAA
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
