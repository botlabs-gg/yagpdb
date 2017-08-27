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
		modtime: 1503833113,
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
		modtime: 1503834503,
		compressed: `
H4sIAAAAAAAA/3Jydff0s+bicgnyD1AIcXTycVUoys9JjU/Oz81NzEspxiKVXpRfWgCScPb39fUMseYC
BAAA///vBqwzRAAAAA==
`,
	},

	"/assets/migrations/1503834503_initial.up.sql": {
		local:   "assets/migrations/1503834503_initial.up.sql",
		size:    692,
		modtime: 1503870813,
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
		modtime: 1503870901,
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
		size:    3576,
		modtime: 1503834503,
		compressed: `
H4sIAAAAAAAA/+yVz2oCMRDG7/sUIWefwGuPBSnFm0iI7uwyMMlskwloxXcvu6jdFNdSil0tXnaHfHzJ
L39mZlcoped2RRD1VC0KpZTadV+l9Mw60FOlAxOYNTtnfRn15Cg/MSXnP319b+bH8mTqxufbphuPENBS
rr0EdDZsn2Grp0pCgkx9hQoC+HVr94koE2css0R08J2U/eQy3TqAFSiNlfOUgg6iWNfI+wXUylK8Pmtq
yrthrRNSaYbufoU1ehkd0rf/80cJm/HxusyrA6fmWgfZX/ZYCvKVYzb3Ke8Pid2T9gM77gB+tOWbfjIB
3hIGMC1ovES6WI7OirXnO0FtOKIg+3Hv/hAtix7y+X74JTf+Szd8VO3f4z0qxFVQHZe33RlcIsGGwDi7
uRNQHLnefgMa0dcExiZhI1y3MVfVADIzgfW3wnwsAuyHHu1f8eY9rWij/UcAAAD//xqryRf4DQAA
`,
	},

	"/assets/settings.html": {
		local:   "assets/settings.html",
		size:    16590,
		modtime: 1503871301,
		compressed: `
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
3ef/AgAA//89SsDVzkAAAA==
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
