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
		modtime: 1501187156,
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
		size:    11837,
		modtime: 1501186422,
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
