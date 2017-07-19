package youtube

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
		size:    9,
		modtime: 1499120954,
		compressed: `
H4sIAAAAAAAA/9JVCPF38VfQVQAEAAD//zTsWAgJAAAA
`,
	},

	"/assets/youtube.html": {
		local:   "assets/youtube.html",
		size:    5472,
		modtime: 1500396648,
		compressed: `
H4sIAAAAAAAA/8xYX2/bOBJ/96eY4xU4G4ik/gHuobUNBEnvECx2220a7O5TQYlji12KVEnKqSHouy9I
UbZjy4rdbRbNg6uK8+c3vxlyRqxrhgsuEUhWflqrylYpkqYZjeraYlEKatulHCkjEDfNaMr4CjJBjZkR
re7JfAQAsPs2UyISy+jFy7Dm1/MX3XJJlxg5e6jJ/I/WJVSlUJTBApGZaZK/2NEs57comQEKBRpDlwhc
AuMmU5rBfY4SKJgqNZnmKTLIciolimDRqUm8hxVnqKZJGdAmjK/C47+iCJJ4gxmiaD4K63scUIHamsBC
q6bVfavwbaTsrJfUgfa/EcMFrYTdkeyV9iRyudyTc3+XjLmwH+pvgz5uMlVs3WPPhzst5x9zdFxrZIxb
WHAUDLiBz5WxYHMESQsEtfDPW7mxVJDoxOVtoZW0ToLbyQXgV1qUAs1rIEtaoCEXQIpKWJ6p0qImntsD
LAuliw61e45WqC3PqCBQoM0Vm5FSGUuAZpYrOSNJViZ1HV9mlq/w/xUXLL65bpqkq/dDH6Hwpun8vVau
dgC5zVFvquvmGt59gMqgdiHH0ySdwzsp1qBkR0BxAVJZSJXN42mq+5385up3rSpYKrDK89a5cPvkAnhL
JmVMozGQUg05NbBQumMPppliOA/BxJkqEocryV/lr95rxSrPgpkmXsxZk95kB94lcF/2GFzuEvcf0+My
oE7urmz09kP6e/Ryffnf7PLLgv30Ttzd/3rgnvvKaQ09onQUzv+UBirX8KVC44HDZ8VlqL6yVNqCQb1C
DTRVK9zs/4NU7+wDX1FLrarySF14BUFTFC4LM7K2UQg+4mx7ol1tKmWaeOkBazvuuSwr+6h/r2VKKnvU
IsqYkqQ3O9PEKT1i19sCuy5xRix+teQBNZmSVitBgLP92P32n5HAQCDg5nqIx8Mz6ZSl75QutwO2CbsL
++EHSJffv0+YKx94b7buPCU/Ur4CZjK/Di0/IH08TQYFZtbH3hnpZ6clIti/6vwNEl/XxcftbNCqfFKl
P4QIkGDEDQu7Xad7DeTWY0NGgDx389Zxwtso/hbnWY7Zn6n6+ijjZxTbxmZg72eULvi3K9RrJZHMIbwB
DK8GYhxwPhRiWlmrZABkqrTg2/pPrYTUyu0sdcnYNGk1emaKxFXEfGhe2v/vk81uV5XWKG3PTGu+2zxX
18+WriLh9QwO5qI+6Q7BgUJX0r1qmsolQnxbpX3ru2jvUQhwP5Epjs1jB3Mfl4JLHJ762ji3w54bBP3w
V5WM2mOzHwyerTlnDCX0HSKuC66oqHBGgqMhD+eei16p7DvEImOp5RlxI2tdxw8P9V9ogU3jZtSjExAM
b7WnQnsA1TH2z4P8Hu0GnrrlwDltZ7thd1pNvEPycByDXQfOS8MJ3QdO7EBwbheCuuYLiPdeNw14RWRQ
1yhZ05zRreCxjnUKPad0LlNlGRpD/Dlz5qF2S93XzrFudxYM5s5wfRYKhgIdimv/7zCOvs47wGDI1zmN
ur2paXt0d59w4gXQ/i2P68elnY+ejReV9FyMJ7W3sqIaOLvxpTmDZ2Py74efRpM3GzE3fB8R9HP55M3I
y27kYiXH7RcDuYCNY6qXwTe0H+bjrcKKivEkFiiXNoc5PJ9A/YCxgDSm1uoxYdzQVCAjF2B1hQGq+2tQ
GDxRdUGFeaDrn5rJqI2m0zsxlk78sUi2MX9DLAPKR6NpJqNp0tXB3gXhQil/axU3zSgU6l8BAAD//1/M
ebFgFQAA
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
