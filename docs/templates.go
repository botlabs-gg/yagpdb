package docs

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

	"/templates/ads.md": {
		local:   "templates/ads.md",
		size:    663,
		modtime: 1502705243,
		compressed: `
H4sIAAAAAAAA/zxSQW7bMBC86xVzKJAEtZ3EaBCgpx7ra9EPrMmVtDXFFbgrK+zrAypxeOBhMTM7M+QJ
hbNDF4ePjKQrFyTuHUFL5gJXjJxmzFTRa9lQQc2hPcqSs+QBlYY5nneQHlUXrJS98TZlH8lhs276GT1z
Ql+YGyBodgqOiXdg8ZELNIMnkoT7f5rJyuuP119DGxyCTg/QgigWtMRD1/3mwncG04kR2UmSNf7Xxp8d
9vi7+c1eNGGmzO0e2DCwG6jokiOeny7bFFfh1TBzwcp8+aIvZYsyFwkMMfTyxhHkeP72GKnuIIiKrI6R
rgzKFVbNedoaowhzcoMtYQQZQpJwgY9Fl2E0SMacKDAq+w6mEL8zUHNt3qpsIlnXw82Pyf/NxvHl6e14
fOnQznf8YTLNH2ybKKWGOXOgxfhTM61UDVcxOSfeQZopJ5dAKVXMauKimeO26gTNqYJS0hX0CYRMragW
qwXeX7nUPeWstX2Eq0TW+2lxjg+Pg/Q3a6dbPx9isu8LTWybTK4+Nm6SC29vd+i67j0AAP//PGczQZcC
AAA=
`,
	},

	"/templates/docs.ghtml": {
		local:   "templates/docs.ghtml",
		size:    700,
		modtime: 1506445083,
		compressed: `
H4sIAAAAAAAA/1xRsY7cIBCt46+YkCYpWLSpfTR3TZroupTRnBl70bHDBmOvToh/j2DxyreNjeC9N+/N
S8nQaJlAGD/MknEVOXe9s7oDAOgRToHGJ/FN6N7C4HCen8SIMKK0PPryH69C98pqePHDciaOGK1n6OcL
8sZw+EYO6ldeMbDlSeg/v157VVD6E3ZEwBB8Vb29KmxuFreBGFdgXOVMg2cjHa3khO6+FFhKAXkiOLz4
4RUnmnOu9CqxJbtf4E5SOsvvokVWZSMqpSW4fwuFD/ju/JUCHH7jmX7kLHRK9Zzz3WFVVGVGc0Js2vT+
q5SgDo+mQcoWTi1Odzfyxuse6rngRKWflCKdLw4jgRguf0+ERsChFGfsuuWpK7xJ724H76Sb5PGn2Dk+
HbfnMkEWPQo13/MSAnFsm7zHPR137J36GcO78VeWb9587CakVMp49hyJ474OZezaYO3cfo8ZR+8jhVvK
tp7/AQAA//8lzjWpvAIAAA==
`,
	},

	"/templates/helping-out.md": {
		local:   "templates/helping-out.md",
		size:    3259,
		modtime: 1505998337,
		compressed: `
H4sIAAAAAAAA/4xWW4vdthN/96eYTR66gXMhbdpA35osvUDbBDZQyhLI2Brbyloa/aXxOeuWfvc/I+nc
kk3po625/uY3lz9/+OntzSuwCVqyfgCHhgC9gYge2gWewzAvgOJW4Ajckmjq87vdwC0D+gVGmgJY6NDD
QKKmMIRInUUhs2madyNF0t+JdhRxgj0uCRaes0rWdnivzmswLYlQXGURw/4rgRF3BMJw73mffXZsVEE4
6181Dazhj2hFfxruZkdeUCz7DbwbCVoW8EQmwc354xXcEkE/RxkpQksT76HnCI41YN8zsIf9o2bPHXZs
KP/4maagP3gW4GwzEIdJbYGxqeNostyP9gH2I4riAbIEThnSIaJzGIFi5Jiz7603cC0ZQIwETnPnHmQk
90wt3b2aF/0CNJACdvT+emu4S1s06VnTPH369BiljPaT9JvmVH1DvfVWaFqAPVUfBYiOXZhsp9VUIFNO
L8e0ymGrYCRM7AETJCmC1KuuoikjZlZYCJF6itDNSdjZv7C1k5UlG0nUzVE/WDGZmO/TqjhREwgTiwZ1
UQQ1qlUlk/1UrIWhn6dpgVnsZP8qbJ69oZjkEG5PKHOklD+UHCHyzhpKm6Z5PVJ3nx+G2RqqtLC9FuSr
zAuhSEmTzH2QHwpDtTr/mylpcNm2InKvjcNnBNCqPIVb6zTa4oP9oQUQuhH9kNPAT9INOFDTwPMN/Mjx
vnZLAwCwhteRUGj7Kw/Wq+5gZZzb+vg77uyAko1qYiHyR+okG4Q7Bfn99SgS0vfbbVHcdOy2H9ljevni
5XbBIZj2WbX2NlIqyPUaRjuLsG/g6w38yTOkkedJ6w/YlmokIkjsSEZNcLL3VLiY8bm6y7m8rRH9jo7g
F4cDvb/++2/lku3gieFunYFaP1+rz03ww5N//nnWwDebmjkgeNpDG9F3I/SRXY7Q0K7++yz4KlrCh9xi
V3evLn7aLwfy9bror4voKaJSjclWDn0WwBvf0YFK7D+RWUF31LyMDwe0vrTbEvJAQfAKlpJvj94jDHaX
P+Olw6u7ilA1+C9JfXNIqssap6RebOCHXqj28nJRZvbF6Qn+1SPxV3md3wkX+FCA/v483pKPFuJDA99u
4DfMTKHaEanm80t/lrNTmfOeCdM8WL9OgTrb2y6vh9Vx2eR5WuaRhw9VVt1uMSWStFXxtbbFxpkP/8nh
QD4vtUtHhk9u8jwWcmFCobR9eHh4OLP+JlBhQW+nDGZpwDJaw5GtZKwcsDwW9lS5F2sVOCPj4wX+TOyM
sxnuXI6KdwlB11/OmJ2zAo5SwoFWYCh10bba0vsDKYw1jwX37boor6vyl+N7TPIsxFPz1mgOs+e7M7aE
eZogUp7DVe8nhha1rcr4i8wCPU+GIlwPXNZ1DzhNpR1tpE44WkoHx7fH8VV5jCEQxhXsrYyAh8J4ehB1
YkVJruJPXrMLurlzNc8ie/IITt+tK/JfBuhC5AyZWxJoMR27ThhuaPeIj5drFat9/mU/n4md+boplafL
up/oWsfNZR1ebuAPtJIXdaQU2CcqY9pRaRtnh1HyBanLIleYvKRSuYxziBw4kTkR1JWicyoAjxyo7H4r
sLfTpPPJURzIXDXlGnrNXqJt5+Phlu/TfAPYlMku5LVtBz5dN3tqy9rvscuX7JxUPUw6k39+99uvJb6W
WZJEDOWi0SySU1Z9xB0qZkHKnql2I23OhktmzmG8HKNkXzZmbx8yz4YVoDGAhxOmuqpT+EDPELnFdlrA
s9h+0QuktzGJXvTHQ8OmNBNwhEQ+X9NOXdfOO7tY1Pxx7pd7fI9J6rAQ6/I66pR9Vl8Ap8Sw55j95B2H
Us6e/3IhN8363ZubN+ui8GpeyvF6OnEPAv8PAAD//9mTr/m7DAAA
`,
	},

	"/templates/index.md": {
		local:   "templates/index.md",
		size:    1066,
		modtime: 1506445667,
		compressed: `
H4sIAAAAAAAA/4xTQWvcXAy8768YkkMSWLyH74NAbyklJbdQeimlh2dbaytrP71K8m7874vsZEloC70Z
S5oZzeh97dnABu8JrTTTSNmTs2TsRfHt7vPjp4/VZvPg0dRMqpR9mHEkncG5kbEM5ISU2/XfSfQAzigq
nZLZFibgPWaZcErZ4YKehgKZHCdl59wFsaEMlCxE5CtHT8aenKK9FZiEhhVlbdhzbhfRnPei4yp5lulK
CYPIIWBjA+8pg6HUyDhSbhcIFzwJ52XeplJEHUZ6JEWq5UjVZnN5iS8UhQB6MJvIXhUERZ+OUeClgBN7
v4DV4tuFlp5TOIOE4A13lHzSHEMmI3kfXwMfCBf3iQdqUVXVxSo3JI7c9R40FAiE1KiYIaGeuu2aFlsj
2uLu8QFjmlGfVRWVeqDRtlgNwH/aoiT1edmSG4L3yc/Sfk6kTIZr3iPl+Sai/g2Kc9TQREYh0HqZhvav
Pl4P0iSnFi9ll4I66c1yKQM5RsIhyykMnxzes1Wbzd3gpDk5H2mYF5omZaTBBFIoI62OQ1bQ7x17P9Uo
qaPXtCODH9e9e7EPu93aUDUy7p4kJ7v9/3Y3p6609Q2WkO8p+aQhv+vI4orOOa/upzelhSMh0wn7lzlR
8FhUjhQvx9aODHpmW05HMm3PixB7TwqLO0wYySyE/9lA0fcr8/uVlYr825pvfFkPZ0juQZApgi5Ke1Kl
Fsa5ITA85LnEYEdLNLmzCr8CAAD//1zYE94qBAAA
`,
	},

	"/templates/quickstart.md": {
		local:   "templates/quickstart.md",
		size:    859,
		modtime: 1506446879,
		compressed: `
H4sIAAAAAAAA/3ySP2/cMAzFd3+Kh1uyXBz035ItRYCgQNEWSDtklC3aZqMTVZE6x/n0hey7Xrt0E0Dy
vZ8e2XyfWJHcSJg5BEwUEhYp+FW4fw4LlAwlwSZCJ9Y2zSNZSbdN86bFg8AEk1nS25ubxY3Jd+3L8nqD
5m2Lr3GdKilRvs48TnbdS46U9+gD98+QiN1nGTnih3Iccc/aS/a75l2LRz6ksCCs5bKWFykZfutBn8lT
NHZB9+iKwUu8MsyS87KvrVeZqsGp/0oxU6dshCjzHjPBC6IYRjK4uECGzeAf4aoTAjpCJs+ZeiMPiT2t
lSPBFZsk8yt5PN09fLv/WAM5Ms3r15XykbJecNrmfYu788xppCZWx3qJkXqrz5XE9b2UaG3z4b9Zlugp
Y6cU6rA7me720C3CU+HCs653dtHOS61g+CJGt3iSgt5FSAwLnPd/WniNB5M7Eg4u1ns5qSXKB1Zlido2
zVkgylw/NPBYMmGenNHf1ucTsClLGafV5ugyS9mOUeGiB72kIJngENgsUI1GaZMDG2QYKFfXTxtdLhEc
TdaFsmohxU/hLTstKUm2M7br5Eh7uGDTSpACOV2vwvJSjU7oUgxqZRjWV12MUhguSB0bnOKwwPhAYEXg
Axv59ncAAAD//9GVsX9bAwAA
`,
	},

	"/templates/templates.md": {
		local:   "templates/templates.md",
		size:    4551,
		modtime: 1506447427,
		compressed: `
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
`,
	},

	"/": {
		isDir: true,
		local: "",
	},

	"/templates": {
		isDir: true,
		local: "templates",
	},
}
