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
		size:    1410,
		modtime: 1503333270,
		compressed: `
H4sIAAAAAAAA/1xUTW/sNgy8+1dMXg5tgdRAmlsuRb+Q9pCHh2Yv7xauTdt6lUVXlNbxvy8oebPd3Ii1
OBzODPeF/QBSdWOgo2dE8az4XmKp0Mk8U+gVpEgTb6DI6Mh77iEBaXKKo6Qf4Mp30NF5l7ZSj+7E2CRH
tRkVeIgyg9Dz4AL38E7TnWHLsohyjyToXeQu+c36XRghOeHTMwUaGX8byKe2ab7+8vTl919BXgUTKQhe
EmSALMlJUJuLjgKysoFG1hRdl0ChxxglL8hLYVl4tU1ze3uLP95oXjwjK4382LzQVnAmOjEIyvHEEatL
Ex4wUFcnGeJKIWFhseYkOJoQtazS2iQXIWs492GdOFRJv4kLLQ4TJYW6QoCD5HECeY+V63xTRuD0sWnw
I36LTInLAg+I2bP+79eHD94NEpEm0X3ZpvksK/jEcZPARaULy1nZn9gE3Yne4DBxZDOY0Ek2ek41s1Yl
SgSUU5HTWBvB+xZfdwN26PcczBKNNwXcn0c0+KnFl6qetQyR2W/4lucFR04r87ts2jQHweDejKsylihH
z7OaTNbaVQUIgVc8FZ93loxZesbriwuj59di22VtLJFPTrJ+kC6JcU01Mi2eDP4Gpt8+UILfqkH3F8X+
lLV8XBkq/sRlFcvyx21sdPj5olXf75aeU1KvprBgvP41BlOvXMGrpb2cmVG7g+5M3bAfqo9M/c7NfN6f
XyHf1bcdhe9SORUK2wcFSpvT/WgobGbgTVMP5jAxejcMHDmkIrCC3xZPdt01B58l8CMOBlEM6IUVQdJU
bjtNHGsauom7f0AjuaDpspgi8r/ZRe6LY64o0J/ZX125jbColz5Dz0uJ3fVCbUlbTYHxqutf+Vjeu3Ah
0TZ4aPGcfXKLdZ39Uq5Mn11wc54Lw2d6K7Wc//IIM89HjqWjTLiCxn8BAAD//4RJYh2CBQAA
`,
	},

	"/assets/migrations/1502731768_initial.down.sql": {
		local:   "assets/migrations/1502731768_initial.down.sql",
		size:    68,
		modtime: 1503322132,
		compressed: `
H4sIAAAAAAAA/3Jydff0s+bicgnyD1AIcXTycVUoys9JjU/Oz81NzEspxiKVXpRfWgCScPb39fUMseYC
BAAA///vBqwzRAAAAA==
`,
	},

	"/assets/migrations/1502731768_initial.up.sql": {
		local:   "assets/migrations/1502731768_initial.up.sql",
		size:    785,
		modtime: 1503322585,
		compressed: `
H4sIAAAAAAAA/7SR34ryMBDFr5unmEuFvkGv/DN+lK/WpXZBWZYQNzEMJBm3TcHHX3RVLFuUvdi7MDlz
ZuZ3pvgvLzMhZhVOaoR6Mi0Q8gWUqxpwk6/rNTTsjLQNd4cWRiIhDa1pSLmzpnwtCnip8uWk2sJ/3KYi
sR05LUnDjiyFeJOlIgnKG4jm2Cs25rOjxsjTnPbS9PaeioRs4KG6Z20GzH3nIh2ckV4dH35TGPhuKVhn
pOoiy8j29Ob9HnbMzqgwoLyuzcH8UIlxJp4z/WDvVdB/RPWW2l1ThQussJxhL9UR6TGsSphjgTXCGnsm
AwN/ndiBW4rEASjE9AznwiYv57h5xEZa0sfTcr3q6EpjnD11+r6x73O5+85FzFbLZV5n4isAAP//ljKc
EREDAAA=
`,
	},

	"/assets/migrations/current_absolute.sql": {
		local:   "assets/migrations/current_absolute.sql",
		size:    785,
		modtime: 1503333542,
		compressed: `
H4sIAAAAAAAA/7SR34ryMBDFr5unmEuFvkGv/DN+lK/WpXZBWZYQNzEMJBm3TcHHX3RVLFuUvdi7MDlz
ZuZ3pvgvLzMhZhVOaoR6Mi0Q8gWUqxpwk6/rNTTsjLQNd4cWRiIhDa1pSLmzpnwtCnip8uWk2sJ/3KYi
sR05LUnDjiyFeJOlIgnKG4jm2Cs25rOjxsjTnPbS9PaeioRs4KG6Z20GzH3nIh2ckV4dH35TGPhuKVhn
pOoiy8j29Ob9HnbMzqgwoLyuzcH8UIlxJp4z/WDvVdB/RPWW2l1ThQussJxhL9UR6TGsSphjgTXCGnsm
AwN/ndiBW4rEASjE9AznwiYv57h5xEZa0sfTcr3q6EpjnD11+r6x73O5+85FzFbLZV5n4isAAP//ljKc
EREDAAA=
`,
	},

	"/assets/migrations/lock.json": {
		local:   "assets/migrations/lock.json",
		size:    3070,
		modtime: 1503322132,
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
		size:    16052,
		modtime: 1503322224,
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

	"/assets/migrations": {
		isDir: true,
		local: "assets/migrations",
	},
}
