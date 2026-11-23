// Package update checks GitHub-Releases-style update manifests. A manifest is
// plain JSON signed with Ed25519; the signature is a separate .sig file
// (base64). Nothing here downloads or runs code on its own: Download verifies
// the installer hash and leaves installation to the user.
package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
	"pixelbench/internal/project"
)

// Channels the settings UI offers.
const (
	ChannelStable = "stable"
	ChannelDev    = "dev"
)

// Repository is the public GitHub repository that hosts signed manifests and
// release assets. Users cannot point the updater at another URL.
const Repository = "Tangerine-Living/indie-studio"

// DefaultManifestURL resolves the signed manifest for a channel from the
// repository's main branch: stable/latest.json or dev/latest.json.
func DefaultManifestURL(channel string) string {
	if channel != ChannelDev {
		channel = ChannelStable
	}
	return "https://raw.githubusercontent.com/" + Repository + "/main/" + channel + "/latest.json"
}

// Settings is the app-level updater configuration, stored next to recent
// projects in the user config directory (never inside a project folder).
type Settings struct {
	Channel      string `json:"channel"`
	AutoCheck    bool   `json:"autoCheck"`
	ManifestURL  string `json:"manifestUrl"`
	SignatureURL string `json:"signatureUrl,omitempty"`
}

// DefaultSettings returns the settings used before the first save.
func DefaultSettings() Settings {
	return Settings{Channel: ChannelStable, AutoCheck: true}
}

func settingsFile() (string, error) {
	dir, err := project.ConfigDir()
	if err != nil {
		return "", apperr.Wrap("CONFIG", "无法定位应用配置目录", err)
	}
	return filepath.Join(dir, "update.json"), nil
}

// LoadSettings reads update.json, falling back to defaults when missing or
// unreadable so a corrupt file cannot block startup.
func LoadSettings() Settings {
	settings := DefaultSettings()
	file, err := settingsFile()
	if err != nil {
		return settings
	}
	if err := fsutil.ReadJSON(file, &settings); err != nil && !os.IsNotExist(err) {
		return DefaultSettings()
	}
	settings.normalize()
	return settings
}

// SaveSettings validates and persists the updater configuration.
func SaveSettings(settings Settings) (Settings, error) {
	settings.normalize()
	if settings.ManifestURL != "" {
		parsed, err := url.Parse(settings.ManifestURL)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			return Settings{}, apperr.New("UPDATE_CONFIG", "更新源必须是 http(s) 地址")
		}
	}
	file, err := settingsFile()
	if err != nil {
		return Settings{}, err
	}
	if err := fsutil.WriteJSONAtomic(file, settings, 0o644); err != nil {
		return Settings{}, apperr.Wrap("CONFIG", "无法保存更新设置", err)
	}
	return settings, nil
}

func (s *Settings) normalize() {
	switch s.Channel {
	case ChannelDev, "beta": // beta is the old name of the dev channel
		s.Channel = ChannelDev
	default:
		s.Channel = ChannelStable
	}
}

// Manifest describes one release. The Windows asset is optional so a manifest
// can be published before the installer is uploaded.
type Manifest struct {
	Version     string `json:"version"`
	Channel     string `json:"channel"`
	PublishedAt string `json:"publishedAt,omitempty"`
	Notes       string `json:"notes,omitempty"`
	PageURL     string `json:"pageUrl,omitempty"`
	Windows     *Asset `json:"windows,omitempty"`
}

// Asset is one downloadable file with its expected hash.
type Asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size,omitempty"`
}

// Status is the UI-facing result of one check.
type Status struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion,omitempty"`
	Available      bool   `json:"available"`
	Channel        string `json:"channel"`
	Notes          string `json:"notes,omitempty"`
	PageURL        string `json:"pageUrl,omitempty"`
	DownloadURL    string `json:"downloadUrl,omitempty"`
	DownloadSHA256 string `json:"downloadSha256,omitempty"`
	DownloadSize   int64  `json:"downloadSize,omitempty"`
	PublishedAt    string `json:"publishedAt,omitempty"`
	CheckedAt      string `json:"checkedAt"`
	Message        string `json:"message"`
}

// Client performs checks with one version and settings snapshot.
type Client struct {
	Current   string
	PublicKey string
	Settings  Settings
	HTTP      *http.Client
	now       func() time.Time
}

// New builds a client; an empty public key makes Check refuse every manifest.
func New(current, publicKey string, settings Settings) *Client {
	settings.normalize()
	return &Client{
		Current:   current,
		PublicKey: strings.TrimSpace(publicKey),
		Settings:  settings,
		HTTP:      &http.Client{Timeout: 15 * time.Second},
		now:       time.Now,
	}
}

// Check fetches and verifies the manifest for the configured channel. A
// missing update source is reported in Status rather than as an error.
func (c *Client) Check(ctx context.Context) (Status, error) {
	status := Status{CurrentVersion: c.Current, Channel: c.Settings.Channel}
	status.CheckedAt = c.now().UTC().Format(time.RFC3339)
	manifestURL := strings.TrimSpace(c.Settings.ManifestURL)
	if manifestURL == "" {
		manifestURL = DefaultManifestURL(c.Settings.Channel)
	}
	manifestURL = strings.ReplaceAll(manifestURL, "{channel}", c.Settings.Channel)
	raw, err := c.fetch(ctx, manifestURL)
	if err != nil {
		// A new channel simply has no manifest yet; that is not a failure.
		if strings.Contains(err.Error(), "HTTP 404") {
			status.Message = fmt.Sprintf("%s 通道还没有发布版本。", c.Settings.Channel)
			return status, nil
		}
		return status, apperr.Wrap("UPDATE_FETCH", "无法获取更新清单", err).WithDetail(manifestURL)
	}
	sigURL := c.Settings.SignatureURL
	if sigURL == "" {
		sigURL = manifestURL + ".sig"
	}
	signature, err := c.fetch(ctx, sigURL)
	if err != nil {
		return status, apperr.Wrap("UPDATE_SIGNATURE", "无法获取更新清单签名", err).WithDetail(sigURL)
	}
	if err := c.verify(raw, signature); err != nil {
		return status, err
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return status, apperr.Wrap("UPDATE_MANIFEST", "更新清单格式不正确", err)
	}
	if manifest.Version == "" {
		return status, apperr.New("UPDATE_MANIFEST", "更新清单缺少版本号")
	}
	if manifest.Channel != "" && manifest.Channel != c.Settings.Channel {
		status.LatestVersion = manifest.Version
		status.Message = fmt.Sprintf("清单属于 %s 通道，与当前 %s 通道不一致。", manifest.Channel, c.Settings.Channel)
		return status, nil
	}
	status.LatestVersion = manifest.Version
	status.Notes = manifest.Notes
	status.PageURL = manifest.PageURL
	status.PublishedAt = manifest.PublishedAt
	if CompareVersions(manifest.Version, c.Current) <= 0 {
		status.Message = fmt.Sprintf("已是最新版本（%s）。", c.Current)
		return status, nil
	}
	status.Available = true
	status.Message = fmt.Sprintf("发现新版本 %s（当前 %s）。", manifest.Version, c.Current)
	if manifest.Windows != nil {
		status.DownloadURL = manifest.Windows.URL
		status.DownloadSHA256 = strings.ToLower(manifest.Windows.SHA256)
		status.DownloadSize = manifest.Windows.Size
	}
	return status, nil
}

func (c *Client) fetch(ctx context.Context, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

func (c *Client) verify(payload, signature []byte) error {
	if c.PublicKey == "" {
		return apperr.New("UPDATE_KEY", "未配置更新公钥，已拒绝未签名的更新清单")
	}
	key, err := decodeKey(c.PublicKey)
	if err != nil {
		return apperr.Wrap("UPDATE_KEY", "更新公钥格式不正确", err)
	}
	sig, err := decodeSignature(signature)
	if err != nil {
		return apperr.Wrap("UPDATE_SIGNATURE", "更新签名格式不正确", err)
	}
	if !ed25519.Verify(key, payload, sig) {
		return apperr.New("UPDATE_SIGNATURE", "更新清单签名无效")
	}
	return nil
}

func decodeKey(text string) (ed25519.PublicKey, error) {
	trimmed := strings.TrimSpace(text)
	if decoded, err := hex.DecodeString(trimmed); err == nil && len(decoded) == ed25519.PublicKeySize {
		return ed25519.PublicKey(decoded), nil
	}
	decoded, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil || len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("需要 32 字节的 Ed25519 公钥（base64 或 hex）")
	}
	return ed25519.PublicKey(decoded), nil
}

func decodeSignature(raw []byte) ([]byte, error) {
	text := strings.TrimSpace(string(raw))
	decoded, err := base64.StdEncoding.DecodeString(text)
	if err == nil && len(decoded) == ed25519.SignatureSize {
		return decoded, nil
	}
	if decoded, hexErr := hex.DecodeString(text); hexErr == nil && len(decoded) == ed25519.SignatureSize {
		return decoded, nil
	}
	// A .sig file may itself be raw binary.
	if len(raw) == ed25519.SignatureSize {
		return raw, nil
	}
	return nil, fmt.Errorf("签名长度不正确")
}

// Download fetches the manifest's Windows installer, verifies its SHA-256 and
// size, and returns the local path. It never executes the installer.
func (c *Client) Download(ctx context.Context, status Status) (string, error) {
	if status.DownloadURL == "" {
		return "", apperr.New("UPDATE_DOWNLOAD", "清单没有提供 Windows 安装包")
	}
	expected := strings.ToLower(strings.TrimSpace(status.DownloadSHA256))
	if len(expected) != 64 {
		return "", apperr.New("UPDATE_DOWNLOAD", "清单缺少有效的 SHA-256")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, status.DownloadURL, nil)
	if err != nil {
		return "", apperr.Wrap("UPDATE_DOWNLOAD", "下载地址无效", err)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", apperr.Wrap("UPDATE_DOWNLOAD", "无法下载更新", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", apperr.New("UPDATE_DOWNLOAD", "下载更新失败").WithDetail(fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
	dir := filepath.Join(os.TempDir(), "PixelBench", "updates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", apperr.Wrap("UPDATE_DOWNLOAD", "无法创建下载目录", err)
	}
	name := downloadName(status.DownloadURL)
	target := filepath.Join(dir, name)
	hasher := sha256.New()
	limited := io.LimitReader(resp.Body, 512<<20)
	written, err := writeStream(target, io.TeeReader(limited, hasher))
	if err != nil {
		return "", err
	}
	if status.DownloadSize > 0 && written != status.DownloadSize {
		_ = os.Remove(target)
		return "", apperr.New("UPDATE_DOWNLOAD", "更新文件大小与清单不一致")
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expected {
		_ = os.Remove(target)
		return "", apperr.New("UPDATE_DOWNLOAD", "更新文件校验失败").WithDetail(actual)
	}
	return target, nil
}

func writeStream(target string, reader io.Reader) (int64, error) {
	tmp, err := os.CreateTemp(filepath.Dir(target), ".download-*")
	if err != nil {
		return 0, apperr.Wrap("UPDATE_DOWNLOAD", "无法写入更新文件", err)
	}
	written, copyErr := io.Copy(tmp, reader)
	closeErr := tmp.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tmp.Name())
		err := copyErr
		if err == nil {
			err = closeErr
		}
		return 0, apperr.Wrap("UPDATE_DOWNLOAD", "无法保存更新文件", err)
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		_ = os.Remove(tmp.Name())
		return 0, apperr.Wrap("UPDATE_DOWNLOAD", "无法保存更新文件", err)
	}
	return written, nil
}

func downloadName(target string) string {
	parsed, err := url.Parse(target)
	if err != nil {
		return "pixelbench-update.exe"
	}
	base := filepath.Base(parsed.Path)
	if base == "." || base == "/" || base == "" {
		return "pixelbench-update.exe"
	}
	return base
}

// CompareVersions compares dotted versions with an optional pre-release
// suffix. It returns -1, 0 or 1. Invalid versions sort as 0.0.0.
func CompareVersions(a, b string) int {
	left, right := parseVersion(a), parseVersion(b)
	for i := 0; i < 3; i++ {
		if left.core[i] != right.core[i] {
			if left.core[i] < right.core[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(left.pre) == 0 && len(right.pre) == 0:
		return 0
	case len(left.pre) == 0:
		return 1
	case len(right.pre) == 0:
		return -1
	}
	for i := 0; i < len(left.pre) && i < len(right.pre); i++ {
		if left.pre[i] == right.pre[i] {
			continue
		}
		ln, lerr := strconv.Atoi(left.pre[i])
		rn, rerr := strconv.Atoi(right.pre[i])
		if lerr == nil && rerr == nil {
			if ln < rn {
				return -1
			}
			return 1
		}
		if lerr == nil {
			return -1 // numeric identifiers sort before alphanumeric
		}
		if rerr == nil {
			return 1
		}
		if left.pre[i] < right.pre[i] {
			return -1
		}
		return 1
	}
	switch {
	case len(left.pre) < len(right.pre):
		return -1
	case len(left.pre) > len(right.pre):
		return 1
	}
	return 0
}

type parsedVersion struct {
	core [3]int
	pre  []string
}

func parseVersion(text string) parsedVersion {
	trimmed := strings.TrimSpace(text)
	trimmed = strings.TrimPrefix(trimmed, "v")
	if at := strings.IndexByte(trimmed, '+'); at >= 0 {
		trimmed = trimmed[:at]
	}
	if dash := strings.IndexByte(trimmed, '-'); dash >= 0 {
		pre := trimmed[dash+1:]
		trimmed = trimmed[:dash]
		var out parsedVersion
		out.pre = strings.Split(pre, ".")
		fillCore(&out, trimmed)
		return out
	}
	var out parsedVersion
	fillCore(&out, trimmed)
	return out
}

func fillCore(out *parsedVersion, core string) {
	parts := strings.Split(core, ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		value, err := strconv.Atoi(parts[i])
		if err != nil || value < 0 {
			continue
		}
		out.core[i] = value
	}
}
