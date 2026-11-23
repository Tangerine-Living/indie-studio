package generate

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"pixelbench/internal/apperr"
)

// TestResult is the outcome of a provider connectivity check.
type TestResult struct {
	OK      bool        `json:"ok"`
	Message string      `json:"message"`
	Detail  string      `json:"detail,omitempty"`
	Text    *TestResult `json:"text,omitempty"`
}

// GeneratedImage is one provider output, still encoded (PNG or JPEG).
type GeneratedImage struct {
	Data   []byte
	Format string
}

// ProgressFunc reports coarse job progress to the queue.
type ProgressFunc func(percent int, stage string)

// Provider is one pluggable image backend. Implementations must honour ctx
// cancellation on every network call.
type Provider interface {
	ID() string
	Test(ctx context.Context) (TestResult, error)
	Generate(ctx context.Context, req JobRequest, progress ProgressFunc) ([]GeneratedImage, error)
}

// NewProvider resolves a provider id against the saved settings.
func NewProvider(settings Settings, id string) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "":
		return NewProvider(settings, settings.DefaultProvider)
	case ProviderComfyUI:
		return newComfy(settings.Comfy)
	case ProviderOpenAI:
		return newOpenAI(settings)
	default:
		return nil, apperr.New("PROVIDER_UNKNOWN", "未知的生成服务").WithDetail(id)
	}
}

func httpClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

// readSnippet keeps error bodies short enough for the UI and logs.
func readSnippet(body io.Reader) string {
	raw, _ := io.ReadAll(io.LimitReader(body, 4096))
	text := strings.TrimSpace(string(raw))
	if len(text) > 400 {
		text = text[:400] + "…"
	}
	return text
}

func validateHTTPURL(raw, label string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(raw), "/")
	if base == "" {
		return "", apperr.New("PROVIDER_WORKFLOW", label+" 未填写地址")
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		return "", apperr.New("PROVIDER_WORKFLOW", label+" 地址必须以 http:// 或 https:// 开头").WithDetail(base)
	}
	return base, nil
}
