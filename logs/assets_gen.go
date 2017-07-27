package logs

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

	"/assets/control_panel.html": {
		local:   "assets/control_panel.html",
		size:    6248,
		modtime: 1501193526,
		compressed: `
H4sIAAAAAAAA/+xZS4/bthO/76eYP/89tEBlIUVPgaxiu5sGAZJtgKQFegoocSQRS5ECSdkxBH33giLl
tdeypW7SIim6B+vBmd88OA+OtusYFlwikLz5IFRZclmSvr+66jqLdSOo9UsVUkZg1fdXCeMbyAU1Zk20
2pL0CgDg8G2uRCTK6NkPYW1Yr56Nyw0tMXJ4qEn62otM4urZAXGTvqGSlgg1GuOuQpUGqGSwoZqr1oCy
FWqwFZelWSVxE7SIGd+E2/9FEcSrvS4QRelVWH9kGxWorQnWeTattp7hacYerDdUooDhN2JY0FbYA8pJ
6sE5bh+O6dzfS5SoqQCD1jrTj4EerD+PnSm2mwBOCqVr0ErgmrhbAjXaSrE1aZR5rPEU+oN35iiDx368
QH7CUmF+n6mPMywAiaAZijkqgITLprVgdw0eoIOkNa7Jbwa1uwux+ULSTCAj0HW8gNWNkgUvV9NEfQ8D
GrKuQ8n6fl4VzxkzbtwV2oALIRmTTMfzIH+oFnIqvXCgA4oZIJDtIQ1sua3AVgjbSnEDuaprKtlqmYzD
DclMlFMhVGvh4TbislAkTbL0Tll8nsRZCq+KoAvVCEoClT5zDeoN6geFMmXdcnjmBtA79Hu3Kt0P10eG
CAEZjhbiBiXwAnaqhdGRfADcqXYU5s2cSJIJU+MFcTSDNLf8NWXEHc/vZzNimujTM0IG3M+YESPkJ2XE
lx8krpBHpVZtMxsm+0D5WdD8XnBjIa+olCgMFFrVR514tPwJdcPKxQoBJFlrrZLAqKWRVWXpuhPTqmFq
K8kBJjjc0F1hJAgcZJFJ+6XENC5qgs+pRkvSJHYv0yT2+ixRHSBpxQiz16hG2S6y3P91Xf3+4aDiFfyg
GsuVNB/qVlhOgNwExQmsrnPLN/iy5YKtxtdA7miNBMjeC8geWMg7FJgPgGSfyG+pNsgO6Eff9P0yy+N2
ScGZL8RPT55LS//OQ4s/ML/xIW1uqPyd4/YWBdqTQn2J9Anl+loItYUa68xV2KGgJrliOJ7hR0FJPLyF
BnXNjXExB1bBhuMWmJc+pqT56kvvP7v5Lzaod0rixW2fJnryhmOA+0L38O+qAEeD3iRPaFp+p0yb1dye
9CrT5jkaM9yLcrhkQuX3BNJ3dIPOxfAuzHj7tgMAn9ETSeyOB+mlCfLxo5+N/UTrpuMpN33CvAu5QKoL
PpU0b9tM8Py4ZTsnu1EhzBKnFh4KaoWINC8rS9IhK6SysPqFa2Pf0hL7PqFQaSzW5CdaWNTrrlvd4RaN
7fv97km6iQSX97Dfxnq4NJrXVO9I6jh0EtM05FJCl7KOwjMslEYn/VfBvPTU3Q2oUwP+5ZnfuhN0pNE0
Shq+wanJf6A5Yjg37lt9IVVslb66TWJbXaa50UgtsnnC69ZWSi8A9IeTBYD+nHOeMInPGdh135TwfH18
wnp1e+Y81HWayhJh9VqV585MM75k6f+7bpCQxJZdpuw6l8jUvuc1wir499ouY115N/c9fLt/cGK/m2d2
GgbnuyPmEnmXC+fwCWrBZ6cjJt8WQ/RWnDGUJNReix/t2CE5I7ChosU1CY4lsGR2Oey6Yy0f3D0E05oU
rRC+8Z3WeJ/hzMWC3ku/DcRLhNMzkI+KRtwMxTF2Udr3sauN8Whk6lq9qx0zbfa0Fxyvn9u8SzkzlMCJ
vjPUmL9UxnybKJSyqC91CZdw8GYHNZX/dYNl/XvhJ/Pj7+KHMk/+VeC3yX9Ovxqj4M8AAAD//3/SUMFo
GAAA
`,
	},

	"/assets/log_view.html": {
		local:   "assets/log_view.html",
		size:    5244,
		modtime: 1501186422,
		compressed: `
H4sIAAAAAAAA/6xYeW/cuBX/fz7FC+PCUmJJ4xS9PMfWR7trINsumqToYrtIOOKThghFaklqxl5B370g
dYxGsR23aP7IUHwHH3/vpOuaYcYlAimrjeDpR4N6h/qjULkhTTOb1bXFohTUIpC0/LhFygjETTNbGnsv
EOx9iSti8c4mqTFkPZsBAPy5QMZpUHAZ7Tmz2wv4w+//WN6FUHuy+2fpRuDoe9iLBL1Xlb3I+B2yxRE9
edVqO5/Pf7N4lRzRMiVtZPiveAHz+E9YHEvulWbRRiP9fAH+J6JCHFiaYTUsXhYmj1IloB6dCaMbnc/n
5d2igeTVO7SWyxzsFsETQWWQKlEVEn4bj+x8aXmBR0p/53U8reL8SAWt7FbpsZI382coeXOsJLVcSTO5
39e1DLdpAfP/xQwFWmRRgcbQHA8uTZVQ+gJ078Zmtkx81KxnS8Z3kApqzIpotSdrzzDeTZWIRB6dv+lo
U3pJc4xcOKIecXiuTOkC2iuuSFJQSXNM6jq+TC3f4bcVFyy+vWmaRKg85zJPskqI9hYECrRbxVakVMZO
FHvlXJaVHYU9AUkLXJHbG9KbtuWMoSSwo6LCFanr+K3KjT/yIY3b8/X1lkqJAlzawcTSv9ECmwaWpqBC
rJcUthqzFUnIenMPP15++8PNVXx3/+syoetl0jLVNc8gvjWXrODSyY6NNtWm4HYwdmMlbKyMRO5/GJU5
6sH2mw6VZF3XKFnTLJPt+QTuxOE9clLC+K7z52j5IoogiQevQhStZx19WmSoQG1NV2b+D4HikPX/Rwwz
WompX7/g9nHFZT7he9l7snNX55mg+7y9uYAJh3N5OEFrgGTYaqE5OtnD86iRbZ3UaEolDd/hQ1HV1tex
QFdet2qHulsbq3mJjABnKyJUHvntB9S1Kp1xD9Naun6c2CnwB/VVkKzf8wIh+PD+OoQl743NKGQ0SqlG
G1UlWS8Tvl4mdvs85Yf6SNaXfv182a7kk/X3bSnzkpNkep4Rh/pK1pftR6fMZ9HjGCaPgejEPfyP+Gaj
2P3jt6zrk+4GcLF6zm3q+uSayn9y3LclgHm5460nxbUrJHDCz+Bk52V9XnTAmqcgsHoIR632UV2f8KYZ
6lWpShfAkdU8z1FDx0aAUUsjq/Jc4MDV77a8K5IKnn4mYLl1TJ0xwGWmOs5S0BQLlHZFrCoJtLupktbv
tQEFXaa3Xy7HoVfVUbrPRyr+8W3Zuq5jlwjG0qJ0FdY+kWQjmfb4Dwa19IXo5bB3w02qefFMXYfQRyGi
um4bFbRxP7i6x3/S7R2fD+k2T5SGQCo7yIVwMo2Zuo6vWzzdGoXBpnm/5QY6jbClBjaIEjQWaocMMq0K
3xpjUFLcA3WxayClEgwicBsPzckjwzM4eW6+dvxjm5tmuamsVXIApv08QDNpnaY4ap1SpUpmXBegpA+4
HrQuKoLTTtFpSNbtmcukPWJ9fJH/tVaAz8DHpZfJI+VimfgGMG3wRy1r+jnqXr5rjXj6n5ZFq33b9V1w
lnY921Gfvub2BlYwnpIWs6ySvmrCMXKFycN2vuRZ8KKDOSCXGuFeVWCqbrGn0oJVnTTYUXh982/5o2Pd
qkqwnoFb4BIYN6nSDLg1KDInX9DP2Grl9tRAriS20UhFO6zFJAwPA69GW2nZD7uzdusXWIHEPfzr+7ff
WVv+A3+p0NggXPT0mDL2lx1K+5YbixJ1QISijJxBj0KAO3sGlNLRWTyDwAkbS21l4MUK3szn4eRB5Sep
gLxTbqp1Y8UepYW9VjK/APLayXdTBIaj15BLymNFJwF5OZQI8tr5wZl97RIh+KImhIsHhbs06sTb7A7C
6SuseRIZ1FrpMTQjSLrbXkrwXKDStNLIenPGilWJMiA//P3de3IGZGttaS4S90b4ThnbNM95NBQm794M
4aL1tYtn1y86h/9V6eKGWtpf0ZFigzYgb1V+e0POuuD/gjx0D3IGDqqD2QYlCxxjuJg1M59Afk76UMIK
Tp8eoU7hwH+j9vJxCab28kjGDWxX1KATGY1sp55olLbIPpR7qpmBFVhd4WJ2Ekw95KJgmPzC2FfGL5jA
PxulUQJjofKg7dgHH8L4iT49+cXRxmJ2lCtHtGmeTGzb2kIEw6Vf9xhPwtppfO/KZeDED9Nz+NP85zMP
Q/i1lPramc5P/9WpGRVmfGwX+LMmnB1q6kGFFz0DjTvUBntUvMs3zpWOHNsrxTgarz9JoDIIn7p585Mr
kjyXSvs9P6B+AiqZ+8qUsutPoNXeHP7qo2EFl1rT+7jUyir3JI2N4CnGKRUisBvXJswZzEN/mHu2ug3g
0iqgTvDg1c7DXqfVsbvWEFAQ0DPYhFA7LY5ybIe7IpcM7y5hBTTmLNboZ79gmDtdYRiH3SByBSvYPFuk
pNogc8f41a20QXvwg3xXX/BdPRT5baPp/QbfDKdEg56LYdXvXR6XwUzpgMMK5gvgsHQACpS53S7g9Wse
gt3EtCxRsustFyyw+if+c7hwaLbbgDTdOlBd51SaoXYVaZn0zX3ypnfBgLr7k2I7mfwnAAD//yeAcYx8
FAAA
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
