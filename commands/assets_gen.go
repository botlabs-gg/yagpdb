package commands

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

	"/assets/commands.html": {
		local:   "assets/commands.html",
		size:    11008,
		modtime: 1506357034,
		compressed: `
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
`,
	},

	"/assets/help-page.md": {
		local:   "assets/help-page.md",
		size:    4394,
		modtime: 1505998337,
		compressed: `
H4sIAAAAAAAA/8xXX2/bOBJ/16eYQx7ORl05adHDIcAhSONeUex2W6Qpin0LRY0k1hSpcii7XvjDL4aU
LNlRk13sSwukpsjf/EjOf56dncFbNOiEvkxubF0Lk8MerrUShAR7+NB4ZY3QcO1K/l4hSafCZPI8/oM9
PBhNfScV6gb28NvyGvYwk3G3OezhU2W3BGG5sA6E1mAdWINADUpVKAkdOk2U2SiPsAdlNgtQB75bpMYa
QtgqX4GvEDLrYYsZMVwrsw7cQVzw8ZmqsAf5KRZmYJCrO4mE9XVnraZpbd3it1ahw7xX1z9S31P6lEJL
2INcAI9aLYJeZttKePB2mJwfrnfTTVkHL569+N+rpFGmPFHBlCIaa0rgyzLeqxrThLzwNCkarUnoNugg
wmbW6B2oApo200p2s8IhoBGZxnyeJtoGDTW0gEaQx+4nU2YB2pZjx2mN5zvdOBQeCURYt0WwuqyEMagJ
aiQSJRLMavE9hYvz83mabCureJdtZUU9OM+sJXRG1Dh448juIDLbeijVBg3UWGfo0qSXYLY2DBbQmscY
+TowiNnihNEoue4ZzdNMA/wBk7MaT4wy47me5a3aIOxs6wh1AQIC3jrQinyIPrERSrNhwhKlicNambxm
1jhkSnaEOcw6RQ9e9klWmLc6WCai0aXw5ruoG42XcH8gu6hentfKBEfY2RbIK95cqw1e3febopt2s1/5
sLYI9wAhPd9pkAhuKwy8W6WJfJqIRqLRV030ptY5NL73qjEtfGBcVHm3330tjCix8/17aNDVikhZQyCF
YeODrxQN2SxH3W/Mqq1HH7N3q0GlK9TIrs5nOmAOmS5an68aEtR7m6MLnvszZKlM/MCXZw4FWcPD18LQ
iQ+vlVw/KfeLkutTwbr1+APBWpnWI83HFO955jS2zSMkg+jnADsVdthY56eER6K3AXQqKjUGZfGvW4DU
R+nu6Bg3DKU+43nelrmHbdl7RhvG2es8X77JlYfa5pwzO6HC2XrkRFvhJk32gO4LA4+uEKaUKWNyJ7HB
PPKFuUmFjoPwgDxJaUfhjLnyHfDkurcn5wtXjTydQIi4QTjfPhZoPW4izoKFRveSejt9L7aTo5BUD/CR
unvLx7hVJJf/b3+KqHW4QUch3y/A4Sbke/zu56MOISJYMbwSb5QmWxS+ChksqERbGVLR/Kg/iPrtkNyX
DertBVL2VriqQRQeI6BfChI1eqfkuK5sIUNXormq79PE20bJyXwf+93YOMjYKzVKpkmX67mqsUX5dwEl
9hOdo/HXH9ZwOOSovYCwrAxUXE+HGv357gZipySFL4TkjCALbtV8+I+nTuvRTTedwhc+F2rCq38lIt8o
+aCeF9ZdhVqOHgRETJr4ytntKXTslXcMIHDC5LYG8m1RgPBgULhsBw3aJrYCrJaDSUZ9hT4lN20IUFsA
qRyDAm4ZJiDnE8Gn0MHvwFhfcTCx6f7D2Jw7Juu6Fn8HAjqqYFxu2QJhsGQsqNPlO3aqOQ1h6m0DF+fQ
C/VvAcWlnc1M3oYbcXwFxc6+HqWNQ19pIMJCwIa2mrsU1gkofhnAV4rOWF9C5X1Dl8tlrkhal4umSaWt
lzluUNsGHS1zK2npkGzrJNKyaynOwhbPbfYVpU8TL9bokJ9JzxfgHf+Fr+MadKgJd2LNB92KXWeqwRws
9zDTwAoL0WrfAxXBRZowJO7zbAGl47/Hdg3949R+3v6V3SL3D100xk9o644SQ1cK+paMWThHOmHWwUkm
aG1REPqBNHjGqyDaN242NnrRWdKE0HDUGg5Xij994HfZb2WRYIAJI/SOdgeieNaQEK2Lt9Dcrr8aHiTa
mjJkNGHgJWytyylN/puJUXDNvrVIx2nzi6Lc1mlCtjV5ZoVj96VsEAkLvRI/arE7tPQjkTCkruCsMGvL
6WrzN4pJshMlP+haljsMosJ/v377cfX639TNH5IsVfH8sh909hm13HEltMvdW7IL4Gi5Ar2MZabGOnxM
s8TzdI/DzoU62fhQyrNyVP2ztgTRehueRIJIlYbNnCZ/BgAA//8eNE/WKhEAAA==
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
