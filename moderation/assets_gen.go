package moderation

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
		size:    995,
		modtime: 1501287538,
		compressed: `
H4sIAAAAAAAA/3RTTW/TQBC991e8I6gpLdeeqJDggLggJIQQUifesb3qesfamU3if49m7USkgks+vDvv
a56/j4xJAheyKBk9k9XCioWGOewxFznEwIqojzc3uMNXCUkG2EiGJIOCUoJdg1DnX9oe78Vg9MKKGwC4
hY1RMdGCPaPwJAcOiLnd7atz+7+eDlIgPaiG2IgwxVKkxDzc9jEZ+y/X85mz03LAxKo08KrKVeTFRW1q
NvZPUsAnmubEj4g9Fqmoyo3++W5PGR9UJpbM2HPMA9TqHMMzOpkmysEv5ouxY0wJlFSwZsJIpIb3Dw9n
NXr21o2UM69RnbGOpOATd9U47MAHzoi5SzU4ceDEf9lSN/sldi/wSRe6oWhzhls8JRulDuPOOZRbxk3b
/4LeQQqyWBvA8O8g+yLTlYGoUHPfVbmvKb1rrajGFx2qcdi2P1VjFEmMXgoIxtMshcqCUNeynKd+SsVI
B4YJlA11fgWwSC3KqXe6j4kp3/tnOc9/aw51XcDpktoOl7Zgv7jm4mvXmbvYRw6O9oNKjnnQawM49+C4
HeM4xm4EFYYV6l44OOJWhTVDqibbeyDFyXStu5svNbHiGH1J1so51xx1nDibthB/PWXwyThrPLzCWuGD
dCvWdta0JOna1kYu/PvNvd+5v5p9+ycAAP//yBsYreMDAAA=
`,
	},

	"/assets/moderation.html": {
		local:   "assets/moderation.html",
		size:    11884,
		modtime: 1501987253,
		compressed: `
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
7/5vAAAA///u0ho9bC4AAA==
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
