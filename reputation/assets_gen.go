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
		size:    2751,
		modtime: 1496105504,
		compressed: `
H4sIAAAAAAAA/6xWb2/bthN+r09xZfP7Qa4jyS6wf7GlrVuLokOWDWn3ahgKWjzZXGhSIyk7maDvPoiU
LVl1tgaYX1gU757j8bnjQ9U1w4JLBJKXHzWWlaWWK/lRIGWoV4pqRpomCOra4rYU1HrPDVJGIG6aYGns
g0CwDyWmxOK9TXJjSBYAAHy3RcZpuOUy2nNmN1fw1Zdfl/cTqJ25/Vm6Ejh4P85Fgj6oyl4V/B7Z4sSe
vPDR5rPZ/xYvkhNboaSNDP8Lr2AWf4PbU+ReaRatNNK7K3CPiArRuzTH0XHwvFQmypWAerAmDHY0n83K
+0UzQNAdtVSfgL5wPr1LZVBLusWh08vZyKlUXNrx4kcX/+/+YoYCLbJoi8bQNfZs5koofQX6wGATLBNX
ryxYPosiWDK+g1xQY1Ki1b4r23A2VyIS62j+srON7SVdY7RxzTLwcF5clpUdNAaBdsspefeaHNAbzhhK
AjsqKkxJXcfXam3id6+bZhxtM89ufXsqLuG6b08DhdJQ1/Gr3PIdvq24YPEN3WLTLJPNfJB2wviu22E3
jKLMvbZcJPFxr4G3t+a283kBUlmIb7F8j9ZyuTbxG9k2KWsPQJ8ZVxIYN84CSoLdcAMG9Q61T6WuURhs
mvFxogK1Nd2B+g9qIlGA+48YFrQSdszm2NuVkMv1yG9I82mAnsvjlCfxJJxn8LGV/TnXaEolDd/haHHn
7vVhCOjkYaN2qLuxsZqXyAhwlhKh1pGbPhPOh2yTO2/zdv24sQvgFuqUgWS3VN4tE7v5PFgvDyR75caf
jx3qBsl+NfgEbC8nJPvFjf8Zu0weI6LFOQof4Xel2IMvRd8+UTv5WEUSBzlT/cQVMvvX1ltV1ip5aJOV
lbCyMio131L9QEDJXPD8LiX9BXetKPtJaQznE5K1L7BVGpeJj3RWNGDU5K65Bz6Hh3fRau+7f2lyzcuh
ECZ/0B31syQLgh3VoLG8qba3am8ghdkiCIpK5k5PzqQs+JbbS1BFYdBOvNrzInzWTRyT9e+QDqIvfM1y
JY0SGAu1Dkkbtz2rLQGg1d6Qib8tco3U4i3+WaGxIXn75gO5BJLQkicjuW0FO+kzTQaF/9Zlm5Kpe07J
/31WKZn6wSXISohLGEB++H6yCJoBBye2sNtxS1tJtUEGKfz4/ueb2L2FrejGnajgB7y33W4KpcMWwx3D
wGHZwWOBcm03C+DT6aS/O11Z1B5SuAhJKwqkZ1arfUzLEiULnZFlZBK3pQ19yN/477Gm8m7yNIiXhieC
DprwRJiXgwHoIiTPPzmwk0McrfaTxeDDY9Cw03REZNBAEFyEh/IdCnbu8M1mlzBrqz0J2k8TdybcNSnZ
p7dkoZRF3X2Oeo+/AwAA//8LEzmkvwoAAA==
`,
	},

	"/assets/settings.html": {
		local:   "assets/settings.html",
		size:    5653,
		modtime: 1493477088,
		compressed: `
H4sIAAAAAAAA/9RYS2/jNhC+51dMeelJEtJDT7aA9IFFgW5QOL0HlDSSiPChpSjHhqD/XpCibNmRZXdr
p8nJImc4j48fh/S0bYY5kwgkrZ41Vo2hhin5XKMxTBY16bq7u7Y1KCpOTa9WIs0IhF13t8jYGlJO63pJ
tHol8R0AwHg2VTzgRXD/k5c5eXk/iCtaYGDtoSbxaucenrz7RVTejxZW8SCAXGkwJcI+Zqi3tUERwoJC
qTFfkqhqEs7SqG3Dh9SwNX5pGM/CP37rumi/LuLOf6Kozkj8535QLyIaL6LKZxVlbO0/fwgCiMJdbhAE
8Z2XH2FFOWpTe7T6ZVq99gu+D7xcaQFacVwS+0lAoClVtiSVqs1I8dhYRSVyaFuWQ7jCagAy/F3ShGPW
dU4hKDSibFvkNQ5TGea04aZtUWZdd+Ri0o3bUiaLCd03OZaYviRqc0LVqXOaID8tdzpMVo0Bs61wZBMk
FbgkPkUylz24RZj5LMdk9Dqn44tmAhzR5pLpN0gmKtteAOOeQOc0Pal+nlE/XmJ5FhRaNdWZRbDbLXs+
LSWZNIHdBBI/UoHu1I5OrJPXs/gdmB5vssGNIQcRpkoarTgBlh249iz4yzl7dDNryhtckrY9IMNeY5Lm
B6FM79/VEUyV4pl6la46wjACJqHGVMnsO8GTjUhQz8C38+vB+3U3noZukH8Y4ATdBBqrgArVSEPir3TD
RCOgH4PKLQ/BlNRASiUkCAVbo4wMfUEJ9JVuLchKIqRKCCqz8EZIHwXq8f5KN1/YGh/85DToB0ofAPn4
IRNMusvpcrBq5JiaaXx6LJzVleJ4QSDOpKpcdfGg+bKP3w4r/85q+GQ0kwUQ0nXQR7O/Bh6VxEXUG7zM
e9uKv3cvAAvFc7+6JkCsO/sYGL9G3ByQJ++YzEfZdecRjfokLsC+GmAvkVdBwlX6QvpNrN2pyDUi30Ja
UlkgULlVEn+sfdkGo+wU9AzfvZNOhzVLv3Pid7/B4hV+a5jGzPHZJmsLRKRRqDX+65vrPMsHd/ZEX53s
x8Y/NOdPBHs16r9DIXzDHY0pWvokaGEfXTL+LOVaiVuwadX7vRmhRvY/BafexvuZaPULp+kLZ7UZmGXf
0wVbM1nYh4tFf0Sqa/Jp5PkmBWrC/ofm0+l4Pz2f+lJlKfVexWoUxq3q1bSLz0Kx/69qzYjnRP+lM3HQ
7ppckzTGKOn/YNVNIti+E5AYCYmRQaWZoHrrvnnhfvz79omucRH1NmIAuGLuE9NTU30fsG/IBcGotxfZ
0zEaO0XX8BvULuxCHrYax86PG5S5Usb+Pw37Rq+j/j8BAAD//5B6qxEVFgAA
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
