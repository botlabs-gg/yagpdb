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
		size:    1409,
		modtime: 1504007704,
		compressed: `
H4sIAAAAAAAA/1xUTW/lNgy8+1dMNoe2QPqANLdcin4h7SGLRZPL3sJn07a2suiK8nP87xek/PJ1E2yR
M5wZ6oFjD1INQ6JjZGSJrPhRsp/QyjRR6hSkKCNvoMxoKUbuIAllDIqjlJ8Q/D/oGGIom5+HcGJssmQ1
jNq4zzKB0HEfEneIQcuV9ZZ5FuUORdCFzG2Jm9WHNECWgk/3lGhg/GtNPh2a5utvd1/+/B0UVTCSghCl
QHrIXIIkNVy0lLAoW9PMWnJoCyh1GLIsM5bZWTqvQ9NcXl7icWR0oe85cyqYpGMFP8+RjGzT4PqAz5L4
Fo82t/1HJ6xIUkanWkbOKCMltCO3/4EGCklLlcNQFZn/X0LmzpmEIYmdncTVe9IG0UuuddZ9mV3N98Yc
GvxywENIQ3RevHkHSXHDSCfGdb0f0iuJQ4ObA+6XWMJsVV93WOXK9D6kMC2TM7ynZz/L2UHCxNORs1c4
wrvWqEr+9UzTHBmL0sC3zQNtPpzfJyjnE2esoYy4QU9t9czwVkoFM4sVF8HRIlWPNaQGFTJkTec6rCOn
Gs5vEtIBjyMVhQYnwEmWYQTFiJUrvmVMEPS2afAz/shMhX2Cmzrim683H7bA/Cij6B6bpvksK/jEeZPE
rsgry0k5nlywneiF2ZPZVoXQymL0gurCWpVwx5WLB9NY39bIne3ZW79s1CSZa9iuzxAehi9VPSvpM3Pc
8G2ZZhy5rMwvsmnTPAr68GxclTFnOUae1GSy0rYqQEi84s43ZmfJNflPNXRPbtvr2Jgzn4Is+kG6Isa1
nGNyZ+0vYPrtgG8j+6LY37L6z5WhEk/so9gyfJzGoNOvr1p13W7pOSU1vc6C8fSPL159T54s3C8JvoLu
TEO/P3kxM3U7N/N5v/6u81W921L6ofj+Uto+KOBlQffnh9JmBl58DwAA//9LclhHgQUAAA==
`,
	},

	"/assets/migrations/1503834503_initial.down.sql": {
		local:   "assets/migrations/1503834503_initial.down.sql",
		size:    68,
		modtime: 1503927285,
		compressed: `
H4sIAAAAAAAA/3Jydff0s+bicgnyD1AIcXTycVUoys9JjU/Oz81NzEspxiKVXpRfWgCScPb39fUMseYC
BAAA///vBqwzRAAAAA==
`,
	},

	"/assets/migrations/1503834503_initial.up.sql": {
		local:   "assets/migrations/1503834503_initial.up.sql",
		size:    692,
		modtime: 1503927285,
		compressed: `
H4sIAAAAAAAA/5SS0WoyMRCFrzdPMZcKvoFX6j9/ka5rWbcXUkqIZlwGksx2NwHp0xfBimJo612YOTNz
OF/m+LSspkotapw1CM1sXiL04ki3vaRugJEq2MJAPRsH1bqB6rUs4aVermb1Fp5xO1FFm9hZzRZ23HKI
F9lEFcF4gkjHm2JPH4l70qc7w3no7X2iCm6D5OpeLGWW++Qid460N8cf2xwy7YFD60ibFEVHaU9vORxg
J+LIhIzy27YEulOp8VTlUtyL9ybYv+W478lEstpEiOxpiMZ38fPaSersL4qHWVxYXw3V+B9rrBa4uf4L
I7ZjWFfwD0tsEDZ4syRz8GHOnQwcWe5ondNdr1bLZqq+AgAA//80BTlutAIAAA==
`,
	},

	"/assets/migrations/current_absolute.sql": {
		local:   "assets/migrations/current_absolute.sql",
		size:    720,
		modtime: 1503931699,
		compressed: `
H4sIAAAAAAAA/5SS0WryQBCFr7NPMZcKvoFX6j/+hMZYYgqVUpbVHcPA7k6abED69MVixWCo9W6Z+Wbm
cM7O8X+aT5VaFDgrEcrZPENIl5CvS8DXdFNuoBFHumqkq1sYqYQttNSwcd9M/pJl8Fykq1mxhSfcTlRS
deysZgs7rjjECzZRSTCeINKxV2zoo+OG9OlOex56e5+ohKsgQ3UvlgaW+85Frh1pb46/tjkMtFsOlSNt
uig6SnV6y+EAOxFHJgyQP7Il0A2lxlN139O9eG+C/Zur+4ZMJKtNhMie2mh8HT+vdXW1vUM8nMwl+auh
ApdYYL7A3s8YsR3DOod/mGGJsMHekoGDD6deS8uR5Sa7s9fr1Sotp+orAAD//+B/ah/QAgAA
`,
	},

	"/assets/migrations/lock.json": {
		local:   "assets/migrations/lock.json",
		size:    3580,
		modtime: 1503931681,
		compressed: `
H4sIAAAAAAAA/+yVT4vyMBDG7/0UIWc/gdf3+IIsizeREO20DEwy2fwBXfG7Ly3qNot1kaVbu3jRIQ9P
88skM3MohJBLvSEIci5WhRBCHNpfIeRCG5BzIT0TqC0bo20Z5Ows/2NKxn76ut7Mj+XF1K4v965dD+BR
U669eDTa7//DXs5F9Aky9RUq8GC3jd0mokxccFwkopPvohxnt+m2HnSEUul4nTKigRC1cfH9BmqlKQzP
mlw5GdY6IZWq7+43WKONo0Pa5v96KmE3Pl5bebXn5IZKZHfbcyvIdw7Zty91fyrsjnTsOXELcNeRH/rJ
eHhL6EE1oOEW6Wo9BOt92cTa8lRYHQeMyHbc2z9F66KDfH0ifqmOvzIPn33753jPHjEQq+HysaeDSRTR
ESijdxMBxZE77jegAW1NoHSKrCLXTcxV1YPMTKDtozCf2wDbvkf7W7z5VCua6PgRAAD///VS3V38DQAA
`,
	},

	"/assets/settings.html": {
		local:   "assets/settings.html",
		size:    16730,
		modtime: 1504007699,
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

	"/assets/migrations": {
		isDir: true,
		local: "assets/migrations",
	},
}
