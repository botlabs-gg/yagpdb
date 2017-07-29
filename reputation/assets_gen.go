package reputation

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

	"/assets/leaderboard.html": {
		local:   "assets/leaderboard.html",
		size:    2477,
		modtime: 1499541168,
		compressed: `
H4sIAAAAAAAA/5RWUZPbNBB+96/Y6g7Obs52wgOFS2wo0OnAlMJcyxPDdBRrnYiTJSMpyYWM/ztjyYmd
NCm9PNiSdr9P8u63q+x2DEsuEUhRf9BYryy1XMkPAilDPVdUM9I0QbDbWaxqQa33XCJlBJKmCWbGbgWC
3daYEYuPNi2MIXkAAPB9hYzTsOIy3nBml3fw4utv6scIds7c/iydCxzMD2uxoFu1snclf0Q2PbKnzz3b
ZDz+Yvo8PbKVStrY8H/xDsbJt1gdIzdKs3iukT7cgXvFVIjepTmMDoMruqaW6rhQAnZ+2xfj+nHau17V
ynzKvDKoJa1w6ONODoO4TMYOBUNWLq05AR2Y/dM9EoYCLbK4QmPoAvtYFkoofQd6H78mmKUuW3mbT16C
VBaSe6zfobVcLkzySrahZ21al5P8/iAHYNw4CygJdskNGNRr1LN0OcmD3Q6FwaY5FQkVqK3pZML4GgpB
jcmIVptOIMPVQolYLOLJV53t1F5TiQLcM2ZY0pWwA8+z3nErUy4XJ35vem2bY4KU8fUJ57M4hjQ5ooM4
zoOLO3v1ajS1koav8WRz5+5VPwR0ol+qNepubKzmNTICnGVEqEXsls/Qecr2cOdt3q4vGzsCt1Gvd5K/
dONZapefh+0qgeT3VD58PmxYIST/w+CTttyXCcl/d+NPY2fppUC0OBfCC/GdK7b1qejlE7eLlzKSOsiZ
7Kcukfn/Sm++slbJvUzmVsLcyrjWvKJ6S0DJQvDiISN9236jKPtVaQwnEcnbCVRK4yz1TIO6Ot5tKHIn
7oHP/uVdtNp49c9MoXlth33/b7qmfpXkQbCmGjTWb1fVvdoYyGA8DYJyJQvXT84cWfCK21tQZWnQRr6L
8TJ81i0cDuvnkA3Ypz5nhZJGCUyEWoSk5W1rtQ0AaLUxJPJdsNBILd7jPys0NiSvX70nt0BSWvN0t0te
Fpav8fWKC5b8/FPTpP1J00Hiv3OnzcjIvUfkS3+qjIz84BbkSohbGEB+/CGaBs0gBke2sPviNmw11QYZ
ZPDLu9/eJm4Wtk036ZoKvsdH231NqXTYYriLMHCYdfBEoFzY5RT4aBT1d4JLi9pABtchaZsC6SOr1Sah
dY2ShdfhzYxXi732fFMgYHSRkRsYdXv8yf9KvAlGcEPym+gCGZlZlpMoaXUS9lhN5cMTIfte8USYbxMD
0HVIrj4q5GjPo9Ummg4u2oGQR9lJgIMGguA63Kd1n8hzRTke38K4VUEUtFexqxV3fUr28e1ZKmVRd3++
vMd/AQAA//9vp6r4rQkAAA==
`,
	},

	"/assets/schema.sql": {
		local:   "assets/schema.sql",
		size:    1301,
		modtime: 1500479242,
		compressed: `
H4sIAAAAAAAA/5STT4+bMBDFz/anmGOQWGmlqidOtOuqqAlEhKq7vSADXmrV2Ftjtvn4Ff+CCaQiOQXm
5+d5b4aHB3iKoyMk/qc9geALkOfglJxAs7fGUMOVTHMlX3lZexh/jomfkIkNo+Q2DzuMyoaLIuUFDL+M
l1waOMbBwY9f4Bt5cTF6U1yaOpW0Yh30TnX+i+rdx0enuyH8vt+7GDFJM8EuUpApJaY6wihXShTqrxyB
9ibrfEXPacnfWUor1Ugz9jIRGGn2p+GaFT2nlWCD1NjSh0fHtTDNcmaTcywTNP8teG2uBW9jM8E5RouK
S7snWPaGHQ/jDRNtaqa3z7Oj22m2f7phLqKbBr2Saq4Zbc1RA5AEB3JK/MMRfgTJ1+4RfkYhsbX6fQCE
VrSszdmNd7owNLbZv1DlZvdCla333lrNNKcC0NUGTw5RN5INLq3IEFoJtGay6NNeLQ+L0gOrx036ys9t
T8O2z74WjIbXaKnehThkE4RP5Pm/2aSjjTNE4SK3seh4dyherK9KXqp3aVp5rapadcdD/wIAAP//6j0o
SRUFAAA=
`,
	},

	"/assets/settings.html": {
		local:   "assets/settings.html",
		size:    5004,
		modtime: 1500578026,
		compressed: `
H4sIAAAAAAAA/9xYTW/jNhO++1fMy8t7koT00JMtIP3AokA3LZwfsKCkkUSEIrUkpdgQ9N8LUpQtOYrt
bBN325NNzpDPzDMPP8SuyzBnAoGk9ReFdWOoYVJ80WgME4Umfb9adZ3BqubUDG4l0oxA2PerdcZaSDnV
ekOUfCbxCgBg2ptKHvAiuPvB25y9vBvNNS0wsPOhIvH2AA+PHn4dlXeTgXU8GiCXCkyJcIwZ9F4brEJY
UygV5hsS1U3CWRp1XXifGtbip4bxLPztl76PjuMi7vATSVVG4t+PDb2OaLyOap9VlLHW//1fEEAUHnKD
IIhX3n7CFeWojPZsDcOUfB4GfBt5uVQVKMlxQ+xfAhWaUmYbUkttJo6nk9VUIIeuYzmEW6xHIsNfBU04
Zn3vHIJCIYquQ65x7Mowpw03XYci6/sTiEUYV1ImigXfFzmWmD4lcveKq3PnNEH+ut35MFE3Bsy+xsmc
IGiFG+JTJOeyBzcIM5/lVIze5/X4ojMBTmRzTfcLJhOZ7a+g8SigS55eVD+ecT8dYnUWFEo29YVBcKiW
XZ9WkkyYwBaBxA+0QrdqJyvW2fVZ/mZTT4tscGfILMJUCqMkJ8CyGbRXwZ8O7MH1tJQ3uCFdNxPD0WNR
5rNQluv37gymUvJMPgu3O8LYAiZAYypF9o3kiaZKUJ2h74Dryfv50F6mbrR/N8RVdBcorANayUYYEn+m
O1Y1FQxtkLnVIZiSGkipgAShYC2KyNAnFECf6d6SLAVCKquKiiz8IKZPAvV8f6a7T6zFe9+5TPrM6Ttg
Pr7PKibc4XQ9WRo5pmaZn4ELN+tWcrwiEDelrN3u4knz2z5+ne/8h1nDR6OYKICQvochmuMx8CAFrqNh
wuvQu87m/4cboWF28bBoGgTjlyKxqPbmdTHTaIj3CprrkeESeR0kXKZPZKiXdgsgV4h8D2lJRYFAxV4K
/L/2OzQYabtgEPPhSvR6WGeVdsl888Mq3uLXhinMnHRtsnYviBRWssU3H1KXBT3C2cX7Bl2/XVmnQB8k
sBvsLC8qpDBFW6QEbUKTXdsrNley+oiabQfcm5RtgvXvrdxPnKZPnGkzFs/eAQvWMlHYw9YmNanbe5Zs
gvzhK20B6z9WsmHB2ardaslNwrjFqluG+2eqeMZ8zvR3PghnrwyLY5LGGCn8vVY3ScWOH2CJEZAYEdSK
VVTt3X9euB9/13ikLa6jYY4YAN4x94Xupa7h+WV4BwmCyZNKZNU3aTtH984yul35+DN/4ZmCn74L5VIa
+1kQDu9r7rL5VwAAAP//opW7TYwTAAA=
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
