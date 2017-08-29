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
		size:    689,
		modtime: 1502705243,
		compressed: `
H4sIAAAAAAAA/1xRsY7cIBCt46+YkJpFm9pHc9ekia5LGc2ZwYuOHSyMbUXI/x5h45VvG0DMe2/mzcvZ
kHVMIEzoRsk4i3VtWu90AwDQItwi2RfxQ+jWQedxHF+ERbAoHdtQbrsI3Sqn4S100504YXKBoR0H5IPh
8YM8bKdcMLLjXug/v95bVVD6C9YiYIxhU92rCus0kz9AjDMwznKkLrCRnmbyQjffCizniNwTXN5C9449
jeu60TeJw9njA0+S0jv+FNWyKhtROfuwUITLb7zTugqdc30+xtpkVBGu7YlNbdl+lxLU5XlSkLI6UpPX
zU4+eM1TJgP2VELJOdF98JgIRDf8vREaAZeSlnHzYWLb2y59+u2Cl76X15/iNPHtepRLB1n0KG7+XqcY
iVNd38Pu7Xpin9TvGD9NWFh+BPPv1CHnksBr4ESczhko4+YKq+96PXu0ISSKu8u6nv8BAAD//36TreKx
AgAA
`,
	},

	"/templates/helping-out.md": {
		local:   "templates/helping-out.md",
		size:    3259,
		modtime: 1504009784,
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
		size:    278,
		modtime: 1502705243,
		compressed: `
H4sIAAAAAAAA/ySPMU7EQAxF+z3F77ZBewcQEqKjoKEcEmdjNvEf2Z6Ncns0SWs9Pz9/zxrQQM6CkUNb
xbKk0jDR8fP68fX+drtcPrNDQ3MXy2XHU3yH2sC1LpKCYuM52+gPqKE67y4RLwhCJ+xs2IolkphlqWBL
bK6pdu+HA3WREj3CrolZQrOkdHwkgr3htJzApDYe0WoTfT2Td7arCxby0bX9g5zFoHAZBq5i46FI4o9q
x360WumJEH+Ko/zyKbf/AAAA//9agLxZFgEAAA==
`,
	},

	"/templates/quickstart.md": {
		local:   "templates/quickstart.md",
		size:    859,
		modtime: 1502705243,
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
		size:    3669,
		modtime: 1504009840,
		compressed: `
H4sIAAAAAAAA/5RX3XLbNhO951OcsTNj+/tsJZK/7ybTZiJbTqo2fxO77XSmF4aJFYmYBGQAlKxamsmD
tC+XJ+ksQEoUbaeNLySaOOfgYLFYrC5yQmbgqZwWwhNIZ0oTlEPlSGJiLH4bvv4wOtlzSCvnTYnUlKXQ
0kFoCaUxE1aZysH4nCymhUjJYa583hBKck5k5HrJRa4cpiIjCFU6eIOciikWpuLnjDx8TiiN8zCVh5ms
jcXZOg56SfKh8ri727m721mter1e+N5ZrSCsqbQMclqU5FCoa4LPhX+OyzXhZ0c2fDCm4V4mye4uhjOh
CnFV0CY2UnjxnAd3wZwkWeKVokJiiRG51KqpV0ZjmSxxdHSE+jNZ4nJ7oksswXGvHNm9EOjwug0djzog
JePwdy8bwAtGnN2KcloQRyq1JLzSGQRK0sEJx6+WaIuPFJstlRbe2M48sj2GJTas4Ux40YWL8HLtLiJP
jA8wWxFUy4JyELgynmVDFF9XqpBPz8nOvi2agVfH6H3lp5V3YZaM32M82oK9qyN+H9gKeq2YGv0wVKW1
lQb6kTL1GNiGsS34cHJ9mgutqbjvevjqJ6RxsOv9/VyTvc8w/LqL/dEoTXLo2+B5ThoiRn+irPP4FFAt
sx2XF6okU/nujF6VFNWCVkbeoTQz4gLgzb1lzHNVELTxEKlXs+0ov6XyiuypqfS9aXTFQ5y3PI2DYff3
ff5CVk1UKjhB3tCMiq6OpZtKWZKYtZAoGBpK2sPLPyuvSJ5pPvSyragmYMtc8SxXyACAsbzAQ3hb0dOJ
KByvMqR1XOA35HMk9N6p9Lo5X1ql1yE/o13lUAbQNuOjKcgxZYhCuVAwrSkISu65UOxiPY3MXLjG4atK
p+zGscn6+Z99CvmJwnbyjB/JV1bzibZCS1NiPRrBUqUe17ToYyaKivr8PIjPA5BPWeOUSxaXdjBaGS3s
AvucNqXQC84BpMKRw4L8QZRVOq40CLV9+LrahAEuNQ0y0LzyBcF5q3TWZsU38bbiUMVDUpD3MQ1JpDnm
xkqkYqq8KNQfJJtoSNxi0Va7xX+xiKOObuC8sB7Om+n2YjXNIawVC55BaU8ZWXcY4exmYk1Zk/nSIy1D
WY9StXxeTSYFhRWyuF3vRj0SUt/Vl8AmDnz8z73ldff5Y8Afx6zA5cPB0YysKOq4uHi6+4e8FZMqnh26
pbQKPteNgNKtezrMwzkx1h77bPmQz8omGAcPpk8dB1yRnxNpPAtr79CPYgjUBAMImzlMrZkpSbKHJrVP
rNBprnSWLHEq+FCuL8n7KX0inEpZbrlpCtQE+6nRUnFCHjRNQTNKWm76hCWGWqJDZttBoH8Qvwf19/Fa
7DJM/t6iyzX231HPbipRuC6bbtCrb5neeISd3fpvZ5v8zvhHBDQ9KNDhvyEXiovusAuP/YI0ekObuQP8
f5v1OqS/xQUTO8xsm9nvLJZL6wN7xJvrvPDE7c5qtW4DqcZ/BdHaxM0U36K/WnGDOq08crL0mHDMyHOt
plPyLvkPt2/bved41OBfXK7vm00DV1+t4c5d16jw33h0CGnI6T3fOBEa4xFP0lqF0ti6K7DTHxz3B+sd
RejI56ooYHSxgMvNvOnZWtdGuFP5YgkmlGxkHlx4x8Jjm7vmKi3pNg4/MLoV0l/ZqFRuWogF/2pouhql
OQBhvfwwFw6ZmpFGx0vnhHz5/Ged4l8+/9WZpRUOjZqDgO1oagKazfyqosq0sSEz6x2Mcvyfa2ppW/uJ
xPPv0VTS/rON3LlnobAp6+IZSzX/Fgs/Wp7Iusf+wcyb31Yi7uF4lPR7eCuuCa5qdELX4JrECxLJoIeX
+zzCjchBctzDWDuyHr/jiiaNhZfJ/3o4LVR6DdKeG/nk7wAAAP//xguCnFUOAAA=
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
