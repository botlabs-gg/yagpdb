// Code generated by "esc -o assets_gen.go -pkg customcommands -ignore .go assets/"; DO NOT EDIT.

package customcommands

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
		_ = f.Close()
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

	"/assets/customcommands.html": {
		local:   "assets/customcommands.html",
		size:    13384,
		modtime: 1525679220,
		compressed: `
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
`,
	},

	"/assets/help-page.md": {
		local:   "assets/help-page.md",
		size:    991,
		modtime: 1524119236,
		compressed: `
H4sIAAAAAAAC/2xTy27bQAy86yvm1sRwontuafq4Fm2BoggCmNZS0iKrpbqk/Pj7YrWKGhc+GSZndoZD
6mlSkwGNDANFp6AQ5IizTDBBk5iM878EOcYVtYX1jOaCCj2r8QAf8fvx67dPH+EVKgMfe7IMGgOfkIEN
RewZk7JDK2kGgdyBYsMOalPbbrGfDBKbWRxqlAyR2fnY4cDpfPHgypWRE5mXqIXWyxQcElMIZ1jv4yto
L5NhoNf8khpFR0EiYy82m/F2X1WPIfw3naInBcGS7zpOOQCvecJ5uqWqV2IpUTnftpw4rlDYeWQFJX6o
Ktxhs3kq+M3mAb+89UXgnRxjYFXqeHZisoRyLFhe1zAmbv1pnmXem3I6cMLN7m6H/RmOW5qC3aKVvGl2
uZj51NhE4U2xePqRJXTWmH31HEGrD/3XLVIX3CeJRj7qFWKztK6wPp+oyeux5poi/5koXKN9545PmfDz
XWrllvXtmCdlEFJGYiQzTrHkWkocOx+Xq/SK504+aGmNLze92agPdd1JoNjdS+rq8bWrS7u+3cK32DWk
DOWo3vyBd/mVKAZlW76X3PdxRaAN1GUUOccuW8yoN/PLAquq+iIJgySGYyMfdAtlnrHPxsMYyPItO2n0
5abOP/VSZr39GwAA//9iIgf23wMAAA==
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
