package update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.2.0", "0.1.9", 1},
		{"0.1.0", "0.1.0", 0},
		{"v0.1.0", "0.1.0", 0},
		{"0.1.0", "0.1.0-dev", 1},
		{"0.1.0-dev", "0.1.0", -1},
		{"0.2.0-beta.2", "0.2.0-beta.10", -1},
		{"0.2.0-beta.10", "0.2.0-beta", 1},
		{"1.0.0", "0.99.99", 1},
	}
	for _, tc := range cases {
		if got := CompareVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func keyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func manifestBytes(t *testing.T, manifest Manifest) []byte {
	t.Helper()
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestCheckVerifiesAndReportsUpdate(t *testing.T) {
	pub, priv := keyPair(t)
	manifest := Manifest{
		Version: "0.2.0",
		Channel: "stable",
		Notes:   "新增地图导出",
		PageURL: "https://example.test/releases/0.2.0",
		Windows: &Asset{URL: "https://example.test/pixelbench.exe", SHA256: strings.Repeat("a", 64), Size: 10},
	}
	raw := manifestBytes(t, manifest)
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, raw))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/stable.json":
			_, _ = w.Write(raw)
		case "/stable.json.sig":
			_, _ = w.Write([]byte(sig))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New("0.1.0-dev", base64.StdEncoding.EncodeToString(pub), Settings{
		Channel:     ChannelStable,
		ManifestURL: server.URL + "/{channel}.json",
	})
	client.now = func() time.Time { return time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC) }
	status, err := client.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Available || status.LatestVersion != "0.2.0" {
		t.Fatalf("status = %+v", status)
	}
	if status.DownloadURL == "" || status.DownloadSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("download fields missing: %+v", status)
	}
	if !strings.Contains(status.Message, "发现新版本") {
		t.Fatalf("message = %q", status.Message)
	}
	if status.CheckedAt != "2026-11-22T00:00:00Z" {
		t.Fatalf("checkedAt = %q", status.CheckedAt)
	}
}

func TestCheckRejectsTamperedManifest(t *testing.T) {
	pub, priv := keyPair(t)
	raw := manifestBytes(t, Manifest{Version: "9.9.9", Channel: "stable"})
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, raw))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".sig") {
			_, _ = w.Write([]byte(sig))
			return
		}
		_, _ = w.Write(append(raw, ' ')) // one extra byte breaks the signature
	}))
	defer server.Close()

	client := New("0.1.0", base64.StdEncoding.EncodeToString(pub), Settings{
		Channel:     ChannelStable,
		ManifestURL: server.URL + "/stable.json",
	})
	if _, err := client.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "签名无效") {
		t.Fatalf("err = %v", err)
	}
}

func TestCheckWithoutKeyRefusesManifest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".sig") {
			_, _ = w.Write([]byte("AAAA"))
			return
		}
		_, _ = w.Write([]byte(`{"version":"1.0.0","channel":"stable"}`))
	}))
	defer server.Close()

	client := New("0.1.0", "", Settings{Channel: ChannelStable, ManifestURL: server.URL + "/stable.json"})
	if _, err := client.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "公钥") {
		t.Fatalf("err = %v", err)
	}
}

func TestCheckTreatsMissingManifestAsEmptyChannel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := New("0.1.0", "", Settings{Channel: ChannelDev, ManifestURL: server.URL + "/{channel}.json"})
	status, err := client.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Available || !strings.Contains(status.Message, "还没有发布版本") {
		t.Fatalf("status = %+v", status)
	}
}

func TestDefaultManifestURLAndChannelMigration(t *testing.T) {
	stable := DefaultManifestURL(ChannelStable)
	want := "https://raw.githubusercontent.com/" + Repository + "/main/stable/latest.json"
	if stable != want {
		t.Fatalf("stable = %q", stable)
	}
	if dev := DefaultManifestURL(ChannelDev); !strings.HasSuffix(dev, "/main/dev/latest.json") {
		t.Fatalf("dev = %q", dev)
	}
	settings := DefaultSettings()
	settings.Channel = "beta"
	settings.normalize()
	if settings.Channel != ChannelDev {
		t.Fatalf("beta migrated to %q", settings.Channel)
	}
}

func TestDownloadVerifiesHash(t *testing.T) {
	payload := []byte("installer-bytes")
	sum := sha256.Sum256(payload)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	client := New("0.1.0", "", DefaultSettings())
	status := Status{
		DownloadURL:    server.URL + "/pixelbench-setup.exe",
		DownloadSHA256: hex.EncodeToString(sum[:]),
		DownloadSize:   int64(len(payload)),
	}
	path, err := client.Download(context.Background(), status)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("downloaded = %q", got)
	}

	status.DownloadSHA256 = strings.Repeat("0", 64)
	if _, err := client.Download(context.Background(), status); err == nil || !strings.Contains(err.Error(), "校验失败") {
		t.Fatalf("err = %v", err)
	}
}

func TestSettingsRoundTripAndValidation(t *testing.T) {
	t.Setenv("PIXELBENCH_CONFIG_DIR", t.TempDir())
	if got := LoadSettings(); got.Channel != ChannelStable || !got.AutoCheck {
		t.Fatalf("defaults = %+v", got)
	}
	saved, err := SaveSettings(Settings{
		Channel:     "nightly",
		AutoCheck:   false,
		ManifestURL: "https://example.test/{channel}.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Channel != ChannelStable {
		t.Fatalf("channel = %q", saved.Channel)
	}
	loaded := LoadSettings()
	if loaded.ManifestURL != "https://example.test/{channel}.json" || loaded.AutoCheck {
		t.Fatalf("loaded = %+v", loaded)
	}
	if _, err := SaveSettings(Settings{ManifestURL: "ftp://example.test/update.json"}); err == nil {
		t.Fatal("expected an error for a non-http update source")
	}
}
