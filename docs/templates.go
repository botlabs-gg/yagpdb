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
		modtime: 1501188794,
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
		modtime: 1501187007,
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
		size:    3258,
		modtime: 1501188069,
		compressed: `
H4sIAAAAAAAA/4xWWYvdxhJ+16+osR8yhrPgxIkhb7GHLJDEhjGEMBhcUpek9qi7+naXzhndkP9+qe4+
mz2++FHqWr/6avn7p1/e3rwCm6Al6wdwaAjQG4jooV3gOQzzAihuBY7ALYmmPr/bDdwyoF9gpCmAhQ49
DCRqCkOI1FkUMpumeTdSJP2daEcRJ9jjkmDhOatkbYf36rwG05IIxVUWMey/ERhxRyAM95732WfHRhWE
s/5V08Aa/opW9KfhbnbkBcWy38C7kaBlAU9kEtycP17BLRH0c5SRIrQ08R56juBYA/Y9A3vYP2r23GHH
hvKPX2kK+oNnAc42A3GY1BYYmzqOJsv9bB9gP6IoHiBL4JQhHSI6hxEoRo45+956A9eSAcRI4DR37kFG
cs/U0t2redEvQAMpYEfvr7eGu7RFk541zdOnT49Rymg/Sb9pTtU31FtvhaYF2FP1UYDo2IXJdlpNBTLl
9HJMqxy2CkbCxB4wQZIiSL3qKpoyYmaFhRCpp6gG2zmJbe1kZck2EnVz1A9WSCbm+7QqPtQCwsSiMV3U
QG1qUclkNxVqYejnaVpgFjvZ/xYyz95QTHKItieUOVLKH8qNEHlnDaVN07weqbvPD8NsDVVW2F7r8U2m
hVCkpDnmNsgPhaBanP/MlDS4bFsBude+4bP6a1Gewq11Gm3xwf7QAQjdiH7IaeAn6QYcqGng+QZ+5nhf
m6UBAFjD60gotP2dB+tVd7Ayzm19/BN3dkDJRjWxEPkjdZINwp2C/P56FAnpx+22KG46dtuP7DG9fPFy
u+AQTPusWnsbKRXkeg2jnUXYN/DtBv7mGdLI86TlB2xLNRIRJHYkoyY42XsqVMz4XN3lXN7WiP5ER/Cb
w4HeX//zj1LJdvDEcLfOQK2fr9XnJvjhyb//Pmvgu03NHBA87aGN6LsR+sguR2hoV/99FnwVLeFD7rCr
u1cXP+2XA/l2XfTXRfQUUanGZCuHPgvgje/oQCX2n8isoDtqXsaHA1pfum0JeZ4geAVLybdH7xEGu8uf
8dLh1V1FqBr8P0l9d0iqyxqnpF5s4KdeqLbyclFm9sXpCf7VI/FXeR3fCRf4UID+8Tzeko8W4kMD32/g
D8xModoRqebzW3+Ws1OZ854J0zxYv06BOtvbLm+H1XHX5HFaxpGHD1VW3W4xJZK0VfG1tsXGmQ9f5XAg
n3fapSPDJzd5HAu5MKFQ2j48PDycWX8TqLCgt1MGszRgmazhyFYyVg5YHgt7qtyLtQqckfHxAn8mdsbZ
DHcuR8W7hKDbL2fMzlkBRynhQCswlLpoW23p/YEUxprHgvt+XZTXVfnL8T0meRbiqXlrNIfZ88MZW8I8
TRApz+Gq9wtDi9pWZfxFZoGeJ0MRrgcu27oHnKbSjjZSJxwtpYPj2+P4qjzGEAjjCvZWRsBDYTw9iDqx
oiRX8Sev2QVd3LmaZ5E9eQSnH9YV+S8DdCFyhswtCbSYjl0nDDe0e8THy7WK1T7/sp/PxM583ZTK02Xd
T3St4+ayDi838BdayYs6UgrsE5Ux7ai0jbPDKPmA1GWRK0xeUqlcxjlEDpzInAjqStE5FYBHDlR2vxXY
22nS+eQoDmSumnIMvWYv0bbz8W7L52m+AWzKZBfy2rYDn46bPbVl7ffY5UN2TqoeJp3Jv7774/cSX8ss
SSKGctFoFskpqz7iDhWzIGXPVLuRNmfDJTPnMF6OUbIvG7O3D5lnwwrQGMDDCVNd1Sl8oGeI3GI7LeBZ
bL/oBdLbmEQP+uOhYVOaCThCIp+Paaeua+edXSxq/jj3yzm+xyR1WIh1eR11yj6rL4BTYthzzH7yjkMp
Z8/XHMhNs3735ubNuii8mpdyu54u3IPA/wIAAP//GJFj67oMAAA=
`,
	},

	"/templates/index.md": {
		local:   "templates/index.md",
		size:    278,
		modtime: 1501187299,
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
		modtime: 1501187358,
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
		size:    3665,
		modtime: 1500480479,
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
