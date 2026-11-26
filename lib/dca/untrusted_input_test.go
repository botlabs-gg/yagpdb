package dca

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func requireFFmpeg(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
}

func makeSine(t *testing.T, seconds int) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "sine.wav")
	cmd := exec.Command("ffmpeg", "-loglevel", "error", "-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=440:duration=%d", seconds), out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generating test audio: %v: %s", err, b)
	}
	return out
}

func encodedDuration(t *testing.T, path string, opts *EncodeOptions) (time.Duration, error) {
	t.Helper()
	session, err := EncodeFile(path, opts)
	if err != nil {
		return 0, err
	}
	defer session.Cleanup()

	frames := 0
	for {
		_, err := session.OpusFrame()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
		frames++
	}
	return time.Duration(frames) * session.FrameDuration(), session.Error()
}

func untrustedOptions(maxDuration int) *EncodeOptions {
	opts := *StdEncodeOptions
	opts.ProtocolWhitelist = "file,pipe"
	opts.MaxDuration = maxDuration
	return &opts
}

func TestProtocolWhitelistBlocksNestedURLs(t *testing.T) {
	requireFFmpeg(t)

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.NotFound(w, r)
	}))
	defer server.Close()

	playlist := filepath.Join(t.TempDir(), "sound.m3u8")
	body := "#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXTINF:1.0,\n" + server.URL + "/segment.ts\n#EXT-X-ENDLIST\n"
	if err := os.WriteFile(playlist, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	encodedDuration(t, playlist, untrustedOptions(0))
	if n := hits.Load(); n != 0 {
		t.Fatalf("ffmpeg fetched %d nested URLs despite the protocol whitelist", n)
	}

	open := *StdEncodeOptions
	encodedDuration(t, playlist, &open)
	if hits.Load() == 0 {
		t.Skip("this ffmpeg build doesn't follow the playlist even without a whitelist, so the check above proves nothing here")
	}
}

func TestMaxDurationTruncatesOutput(t *testing.T) {
	requireFFmpeg(t)
	sine := makeSine(t, 6)

	got, err := encodedDuration(t, sine, untrustedOptions(2))
	if err != nil {
		t.Fatalf("encoding failed: %v", err)
	}
	if got < 1500*time.Millisecond || got > 2500*time.Millisecond {
		t.Fatalf("expected about 2s of audio, got %s", got)
	}
}

func TestNormalFileStillEncodes(t *testing.T) {
	requireFFmpeg(t)
	sine := makeSine(t, 3)

	got, err := encodedDuration(t, sine, untrustedOptions(600))
	if err != nil {
		t.Fatalf("encoding failed: %v", err)
	}
	if got < 2500*time.Millisecond {
		t.Fatalf("expected about 3s of audio, got %s", got)
	}
}
