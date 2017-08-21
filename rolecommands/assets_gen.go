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
		modtime: 1503149194,
		compressed: `
H4sIAAAAAAAA/3Jydff0s+bicgnyD1AIcXTycVUoys9JjU/Oz81NzEspxiKVXpRfWgCScPb39fUMseYC
BAAA///vBqwzRAAAAA==
`,
	},

	"/assets/migrations/1502731768_initial.up.sql": {
		local:   "assets/migrations/1502731768_initial.up.sql",
		size:    769,
		modtime: 1503149374,
		compressed: `
H4sIAAAAAAAA/7RR3WoyMRC93jzFXCr4Bnvlz/gRvjWWNQWllBCbGAbyY/cHfPyiXcWli9KL3oWTM2fm
nDPDf1zkjM1LnEoEOZ0VCHwJYi0Bt3wjN1Alb5WrUnusYcQyMlDbirS/cMRrUcBLyVfTcgf/cTdhmWvJ
G0UG9uQoNjfahGVRBwuNPfXAyn62VFl13lN3Q2/vE5aRi2kID8nYAfHQ+oaO3qqgTw+/KQ581xSdt0q3
TVJNcud3Ohxgn5K3Og4wr2enaH+w2DhnzzP9SCHoaP4o1Vtrd0MlLrFEMcdeqyMyY1gLWGCBEmGDPZGB
hb9r7BJGlwUXC9w+ykI5MqfzMT10dHU/zp8qfXvq63Q+71TYfL1acZmzrwAAAP//hOh8mwEDAAA=
`,
	},

	"/assets/migrations/lock.json": {
		local:   "assets/migrations/lock.json",
		size:    3070,
		modtime: 1503149194,
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
		size:    15906,
		modtime: 1503319632,
		compressed: `
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
qP/8PQAA//9yI1muIj4AAA==
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
