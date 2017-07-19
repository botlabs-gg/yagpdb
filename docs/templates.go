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

	"/templates/docs.ghtml": {
		local:   "templates/docs.ghtml",
		size:    683,
		modtime: 1500392475,
		compressed: `
H4sIAAAAAAAA/1xRsY7cIBCt46+YkJpFm9pHc9ekia5LGc2ZwYuOHSyMvYqQ/z3CxpZvGxvBe2/ee5Oz
IeuYQJjQjZJxFsvStN7pBgCgRbhFsi/ih9Ctg87jOL4Ii2BROrah/O1D6FY5DW+hm+7ECZMLDO04IO8M
jx/kYf3KB0Z23Av959d7qwpKf8FaBIwxrKrbq8LqZvI7iHEGxlmO1AU20tNMXujmW4HlHJF7gstb6N6x
p3FZVvoqsSc7LvAkKb3jT1Ejq9KIyvnyG++0LEIfx8PQKqCKZB1MbOqw9ruUoC7PHkHKmkVNXjcbeec1
T9sYsKeyjpwT3QePiUB0w98boRFwKXsybt7tr41t0qfbLnjpe3n9KU6Ob9f9uUyQRY/imu91ipE41eKO
uLfriX1Sv2P8NOHB8iOYf6cJOZfuXwMn4nRuXxk3V1g9199zRhtCorilrPX8DwAA//8f5cgSqwIAAA==
`,
	},

	"/templates/helping-out.md": {
		local:   "templates/helping-out.md",
		size:    3245,
		modtime: 1499120953,
		compressed: `
H4sIAAAAAAAA/4xWW4vcxhJ+16+otR9iw1xw4sSQt9hLLpDEhjWEYAIuqUtSe9VdfbpLO6sT8t8P1d0z
o7F3D3mUuq5ffXX584ef3l2/BpugJesHcGgI0BuI6KFd4AUM8wIobgOOwC2Jpj6/2x3cMKBfYKQpgIUO
PQwkagpDiNRZFDK7pnk/UiT9neiOIk5wwCXBwnNWydoOb9V5DaYlEYqbLGLYfyUw4h2BMNx6PmSfHRtV
EM76V00DW/gjWtGfhrvZkRcUy34H70eClgU8kUlwvX68ghsi6OcoI0VoaeID9BzBsQbsewb2cHjQ7Nph
x4byj59pCvqDZwHONgNxmNQWGJs6jibL/Wjv4TCiKB4gS+CUIR0iOocRKEaOOfveegPPJAOIkcBp7tyD
jOSeq6XX86IfgAZSwI6a5unTp6fAZLSfZdw054Ib6q23QtMC7KmaLbl37MJkOy2gYpdyRjmMTY5UBSNh
Yg+YIEkRpF51FUAZMRPBQojUU1SD7ZzEtnaysmQbibo56gcrChPzbdoUH2oBYWLRmC5gV5taRzLZTUVX
GPp5mhaYxU72v4W/szcUkxyj7QlljpTyh9IhRL6zhtKuad6M1N3mh2G2hioRbK8l+CozQShS0hwz8/ND
4aTW4z8zJQ0u21ZAbrVVeFVyLcpTuLFOoy0+2B9Jj9CN6IecBn6WbsCBmgZe7OBHjre1PxoAgC28iYRC
+195sF51Byvj3NbH3/HODijZqCYWIn+iTrJB+KAg//VsFAnp+/2+KO46dvtP7DG9evlqv+AQTPu8WnsX
KRXkeg2jnUXYN/D1Dv7kGdLI86TlB2xLNRIRJHYkoyY42VsqVMz4XH3IubyrEf2OjuAXhwP99ezvv5VK
toMnhrttBmr7Yqs+d8EPT/7553kD3+xq5oDg6QBtRN+N0Ed2OUJDd/XfF8FX0RI+5Ka6+vD64qd9PJCv
t0V/W0TPEZVqTLZy6IsA3vqOjlRi/5nMBrqT5mV8OKD1pduWkEcIglewlHwH9B5hsHf5M146vPpQEaoG
/09S3xyT6rLGOamXO/ihF6qtvFyUmX1xeoZ/80D8VV4ndsIFPhagv1/HW/LRQnxs4Nsd/IaZKVQ7ItV8
fulXOTuVWfdMmObB+m0K1NnednkhbE7rJU/QMo48fKyy6naPKZGkvYpvtS12znz8Vw4H8nmNXToyfHZj
uEt7IRcmFEr7+/v7+5X1t4EKC3o7ZTBLA5bJGk5sJWPliOWpsOfKvdyqwIqMDxf4C7EVZzPcuRwV7xKC
LrycMTtnBRylhANtwFDqom21pQ9HUhhrHgru221R3lblx+N7SHIV4rl5azTH2fPdii1hniaIlOdw1fuJ
oUVtqzL+IrNAz5OhCM8GLgu6B5ym0o42UiccLaWj45vT+Ko8xhAI4wYOVkbAY2E83Ys6saIkV/Enb9gF
3dW5mqvInjyA03fbivzjAF2IrJC5IYEW06nrhOGa7h7w8WqrYrXPH/fzhdjK13WpPF3W/UzXOm4u6/Bq
B3+glbyoI6XAPlEZ045K2zg7jJJvRl0WucLkJZXKZZxD5MCJzJmgrhSdUwF45EBl91uBg50mnU+O4kDm
qinH0Bv2Em07n061fJHmG8CmTHYhr2078Pm4OVBb1n6PXb5d56TqYdKZ/PP7334t8bXMkiRiKBeNZpGc
suoT3qFiFqTsmWo30m41XDJzjuPlFCX7sjF7e595NmwAjQE8njDVVZ3CR3qGyC220wKexfaLXiC9jUn0
hj8dGjalmYAjJPL5fnbqunbe6mJR86e5Xy7wAyapw0Ksy+uoU/ZZfQGcEsOBY/aTdxxKOXv+zU3cNNv3
b6/fbovC63kpt+v6qi0C/wsAAP//WuEmt60MAAA=
`,
	},

	"/templates/quickstart.md": {
		local:   "templates/quickstart.md",
		size:    591,
		modtime: 1499120953,
		compressed: `
H4sIAAAAAAAA/3xSPW/bMBDd+SsevHhxZPRr8ZYiQJaiLZB2yEiRV4kIfceSR6vMry8oKw26ZBPwPvWO
5sccCpKdCEuIETPFhCYVv2twT7GhkKIm6EwYRQdjHkhrOhnzbsC9QAWzaiqn47HZKflx+NOejzDvB3zj
VVVTonyTwzTrjZPMlA9wMbgnCGP3RabA+FkCT7gLxUn2O/NhwEM4p9gQV7iucJOa4a8cuEyeWION5YCx
KrzwXrFIzu3QqftMPWDj7wsWGktQAstywELwAhbFRArLDfLrGvCfcfeJESMhkw+ZnJKHsKMVuRBs1Vly
eCaPx9v773ef+yCXQMv664XyhXJ5rTOYjwNuXzSbpC/WZU6YyWn/XJtY56SyDubTm1tW9pSxKxS72G6h
uwPKdcINeO2znnexrC9H7cXwVZROeJQKZxnCscF6/48S1nkw2wvhbLm/l80tUT6HUoJwGczfAAAA///i
AytOTwIAAA==
`,
	},

	"/templates/templates.md": {
		local:   "templates/templates.md",
		size:    3665,
		modtime: 1500392276,
		compressed: `
H4sIAAAAAAAA/5RX23LbNhB951ecsTMTu7WVSG5fMm0msuWkanOb2G2nM30wTKxIxCQgA6Bk1dJMPqT9
uXxJZwFSomg7bfwg0dw9B2cXi8XqPCdkBp7KaSE8gXSmNEE5VI4kJsbij+Gr96Pjxw5p5bwpkZqyFFo6
CC2hNGbCKlM5GJ+TxbQQKTnMlc8bQEnOiYxcLznPlcNUZAShSgdvkFMxxcJU/JyRh88JpXEepvIwk7Ww
uFpHQS9J3lcet7c7t7c7q1Wv1wvfO6sVhDWVloFOi5IcCnVF8Lnwz3CxBvzqyIYP9mmwF0myu4vhTKhC
XBa0yY0UXjxj4y4YkyRLvFRUSCwxIpdaNfXKaCyTJQ4PD1F/JktcbC90gSU475UjG/Ic3rY9x6NtHyWj
9YcXjf05O5zeiHJaEOcptSS80hkEStJBB2evZmhzjxRLLZUW3tjtZWTbhCU2oOFMeNHxFuHdWlt0PDY+
eNmKoFoClIPApfHMGjL4qlKFfHJGdvZ1mQy4OkHvKj+tvAurZPwe49GW29s623cdWxmvGVOj73dVaS2l
cf1AmXrI2QbblvtwcnWSC62puKt6+PIXpNHY1f5ursneRRh+3fX92ShNcujbzvOcNETM/kRZ5/ExeLXE
dlSeq5JM5bsrelVSZAtcGXmH0syID783d8KY56ogaOMhUq9m21l+Q+Ul2RNT6TvL6IpNXLWxvAyrv6vz
N7JqolLBBfKaZlR0eSxdV8qSxKzliYJdQzu7P/zT8pLkqeYDL9uMagKWzN3OcncMDjCWAzyAtxU9mYjC
cZShrGOAX1HPEdB7q9Kr5nhplV6F+oxylUMZnLYRH0xBjiFDFMqFZmlNQVDysQuNLvbSiMyFaxS+rHTK
ahyLrJ//W6eQHylsJ6/4gXxlNZ9oK7Q0JdbW6CxV6nFFiz5moqioz8+D+DwA+ZQ5TrhhcVsHeyujhV1g
j8umFHrBNYBUOHJYkN+PtErHSANRW4evu00wcKtpPAPMK18QnLdKZ21UfBNvKk5VPCQFeR/LkESaY26s
RCqmyotC/UWyyYbEDRZttht8i0W0OrqG88J6OG+m28FqmkNYKxa8gtKeMrLuILqzmok1ZQ3mC4+0DE09
UtX0eTWZFBQiZHK73o3aEkrf1VfAJg98/M+85bj7/DHgjyNm4Pbh4GhGVhR1Xlw83f0D3opJFc8O3VBa
BZ3rIUDp1h0d1uGaGGuPPZZ8wGdlk4z9e8unzgMuyc+JNJ6G2Dvww5gCNcEAwmYOU2tmSpLsoSntYyt0
miudJUucCD6U6yvybkkfC6dSpltuBgI1wV5qtFRckPvNQNBYScvNjLDEUEt0wCw7EPT34/eg/j5ak12E
xd9ZdLHG/j/o6XUlCtdF0zV69S3TG4+ws1v/7WyD3xr/AIGmewk6+NfkQnPRHXThsVeQRm9oM7eP77dR
r0L5W5wzsIPMtpH9TrDcWu/ZI95c54UnHnZWq/UISLX/Fzxam7hZ4mv4VyseTqeVR06WHiKOFXmm1XRK
3iXf8PC2PXeOR43/84v1fbMZ3+qrNdy56x4V/huPDiANOf3YN0qExnjEi7SiUBpbdwV2+oOj/mC9owjT
+FwVBYwuFnC5mTczW+vaCHcqXyxBhJINzb2BdyQ8tLlrrNKSbqL5HutWSn9noVK5aSEW/IuhmWqU5gSE
ePlhLhwyNSONjpbOCfn86e+6xD9/+qezSisdGjUGwbfDqQloNvOLjCrTxobKrHcw0vF/rumlbe5HEs9+
RNNJ+083dGeeicKmrJtnbNX8Oyz8YHkk6xn7JzNvfleJuIfjUdLv4Y24Iriq4QlTg2sKL1Akgx5e7LGF
B5H95KiHsXZkPf7EJU0aCS+S73o4KVR6BdKeB/nk3wAAAP//4XpfgFEOAAA=
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
