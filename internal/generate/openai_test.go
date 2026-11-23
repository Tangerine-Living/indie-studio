package generate

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pixelbench/internal/apperr"
	"pixelbench/internal/secure"
)

func testOpenAISettings(t *testing.T, base, key, model string) Settings {
	t.Helper()
	enc := ""
	if key != "" {
		var err error
		enc, err = secure.Protect(key)
		if err != nil {
			t.Fatal(err)
		}
	}
	return Settings{
		DefaultProvider: ProviderOpenAI,
		OpenAI: storedOpenAI{
			BaseURL:   base,
			Model:     model,
			APIKeyEnc: enc,
		},
	}
}

func TestOpenAIGenerateUsesB64Response(t *testing.T) {
	pngBytes := tinyPNG(t)
	var calls int
	var seen struct {
		Model   string `json:"model"`
		Prompt  string `json:"prompt"`
		Size    string `json:"size"`
		RawBody string `json:"-"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/generations" {
			http.NotFound(w, r)
			return
		}
		calls++
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("authorization = %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &seen)
		seen.RawBody = string(body)
		payload := map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(pngBytes)}},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	settings := testOpenAISettings(t, server.URL, "test-key", "qwen-image")
	provider, err := newOpenAI(settings)
	if err != nil {
		t.Fatal(err)
	}
	images, err := provider.Generate(context.Background(), JobRequest{
		Prompt: "红屋顶",
		Width:  1536,
		Height: 1024,
		Count:  2,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 2 || calls != 2 {
		t.Fatalf("images = %d, calls = %d", len(images), calls)
	}
	if seen.Model != "qwen-image" || seen.Prompt != "红屋顶" || seen.Size != "1536x1024" {
		t.Fatalf("request = %+v", seen)
	}
	if strings.Contains(seen.RawBody, `"n"`) || strings.Contains(seen.RawBody, "response_format") {
		t.Fatalf("unexpected compatibility-breaking fields: %s", seen.RawBody)
	}
}

func TestOpenAITestChecksTextModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, `{"data":[{"id":"img-model"},{"id":"chat-model"}]}`)
	}))
	defer server.Close()

	settings := testOpenAISettings(t, server.URL, "key", "img-model")
	settings.OpenAI.TextModel = "chat-model"
	provider, err := newOpenAI(settings)
	if err != nil {
		t.Fatal(err)
	}
	result, err := provider.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Text == nil || !result.Text.OK {
		t.Fatalf("result = %+v text = %+v", result, result.Text)
	}
	if !strings.Contains(result.Text.Detail, "chat-model") {
		t.Fatalf("text detail = %q", result.Text.Detail)
	}

	settings.OpenAI.TextModel = "missing-chat"
	provider, _ = newOpenAI(settings)
	result, err = provider.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Text == nil || result.Text.OK {
		t.Fatalf("expected text failure, got %+v", result.Text)
	}
	if !strings.Contains(result.Text.Message, "文本模型") {
		t.Fatalf("text message = %q", result.Text.Message)
	}
}

func TestOpenAIRequiresKey(t *testing.T) {
	settings := testOpenAISettings(t, "http://127.0.0.1:1", "", "gpt-image-1")
	provider, err := newOpenAI(settings)
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Generate(context.Background(), JobRequest{Prompt: "x", Width: 256, Height: 256, Count: 1}, nil)
	if !apperr.Is(err, "PROVIDER_AUTH") {
		t.Fatalf("err = %v", err)
	}
}

func TestOpenAIUnauthorizedMapsToAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"message":"invalid api key","code":"invalid_api_key"}}`)
	}))
	defer server.Close()
	settings := testOpenAISettings(t, server.URL, "bad", "gpt-image-1")
	provider, _ := newOpenAI(settings)
	_, err := provider.Generate(context.Background(), JobRequest{Prompt: "x", Width: 256, Height: 256, Count: 1}, nil)
	if !apperr.Is(err, "PROVIDER_AUTH") {
		t.Fatalf("err = %v", err)
	}
}

func modelsServer(t *testing.T, ids ...string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		data := make([]map[string]string, 0, len(ids))
		for _, id := range ids {
			data = append(data, map[string]string{"id": id, "object": "model"})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data})
	}))
}

func TestOpenAITestReportsModelMissingFromAccount(t *testing.T) {
	server := modelsServer(t, "gpt-image-1", "qwen-image")
	defer server.Close()
	provider, err := newOpenAI(testOpenAISettings(t, server.URL, "key", "gpt-image-2.6-flare-c"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := provider.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.OK {
		t.Fatalf("result = %+v", result)
	}
	if !strings.Contains(result.Detail, "gpt-image-2.6-flare-c") || !strings.Contains(result.Detail, "qwen-image") {
		t.Fatalf("detail = %q", result.Detail)
	}
}

func TestOpenAITestAcceptsListedModel(t *testing.T) {
	server := modelsServer(t, "gpt-image-1")
	defer server.Close()
	provider, err := newOpenAI(testOpenAISettings(t, server.URL, "key", "GPT-Image-1"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := provider.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %+v", result)
	}
}

func TestOpenAIModelNotFoundSuggestsAvailableModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			io.WriteString(w, `{"data":[{"id":"gpt-image-1"},{"id":"qwen-image"}]}`)
		case "/images/generations":
			w.WriteHeader(http.StatusServiceUnavailable)
			io.WriteString(w, `{"error":{"message":"No available channel for model gpt-image-2.6-flare-c under group default","code":"model_not_found"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	provider, _ := newOpenAI(testOpenAISettings(t, server.URL, "key", "gpt-image-2.6-flare-c"))
	_, err := provider.Generate(context.Background(), JobRequest{Prompt: "x", Width: 256, Height: 256, Count: 1}, nil)
	if !apperr.Is(err, "PROVIDER_MODEL") {
		t.Fatalf("err = %v", err)
	}
	var target *apperr.Error
	if !errors.As(err, &target) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(target.Detail, "qwen-image") || !strings.Contains(target.Detail, "HTTP 503") {
		t.Fatalf("detail = %q", target.Detail)
	}
}

func TestSizeForMapsAspectRatio(t *testing.T) {
	cases := []struct {
		w, h int
		want string
	}{
		{1024, 1024, "1024x1024"},
		{1536, 512, "1536x1024"},
		{512, 1024, "1024x1536"},
	}
	for _, item := range cases {
		if got := sizeFor(item.w, item.h); got != item.want {
			t.Fatalf("sizeFor(%d, %d) = %s", item.w, item.h, got)
		}
	}
}
