package generate

import (
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

func TestGenerateTextPostsChatCompletion(t *testing.T) {
	var seen struct {
		Model       string  `json:"model"`
		Temperature float64 `json:"temperature"`
		MaxTokens   int     `json:"max_tokens"`
		Messages    []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer text-key" {
			t.Errorf("authorization = %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &seen); err != nil {
			t.Errorf("payload: %v", err)
		}
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"  一盏提灯。  "}}]}`)
	}))
	defer server.Close()

	settings := testOpenAISettings(t, server.URL, "text-key", "gpt-image-1")
	settings.OpenAI.TextModel = "my-chat-model"
	got, err := GenerateText(settings, TextRequest{
		System:      "你是像素美术提示词作者",
		User:        "  名称：旅人  ",
		MaxTokens:   5000, // clamped to 4096
		Temperature: 3,    // clamped to 2
	})
	if err != nil {
		t.Fatalf("GenerateText: %v", err)
	}
	if got.Text != "一盏提灯。" {
		t.Fatalf("text = %q", got.Text)
	}
	if got.Model != "my-chat-model" {
		t.Fatalf("model = %q", got.Model)
	}
	if seen.Model != "my-chat-model" {
		t.Fatalf("payload model = %q", seen.Model)
	}
	if seen.Temperature != 2 || seen.MaxTokens != 4096 {
		t.Fatalf("clamps: temperature=%v maxTokens=%d", seen.Temperature, seen.MaxTokens)
	}
	if len(seen.Messages) != 2 {
		t.Fatalf("messages = %d", len(seen.Messages))
	}
	if seen.Messages[0].Role != "system" || seen.Messages[0].Content != "你是像素美术提示词作者" {
		t.Fatalf("system = %+v", seen.Messages[0])
	}
	if seen.Messages[1].Role != "user" || seen.Messages[1].Content != "名称：旅人" {
		t.Fatalf("user = %+v", seen.Messages[1])
	}
}

func TestGenerateTextReadsContentParts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"choices":[{"message":{"content":[{"type":"text","text":"像素风"},{"type":"text","text":"、透明背景"}]}}]}`)
	}))
	defer server.Close()

	settings := testOpenAISettings(t, server.URL, "key", "gpt-image-1")
	got, err := GenerateText(settings, TextRequest{User: "写一条提示词"})
	if err != nil {
		t.Fatalf("GenerateText: %v", err)
	}
	if got.Text != "像素风、透明背景" {
		t.Fatalf("text = %q", got.Text)
	}
	if got.Model != "gpt-4o-mini" {
		t.Fatalf("default model = %q", got.Model)
	}
}

func TestGenerateTextNamesTheUnavailableModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error":{"message":"模型不存在"}}`)
	}))
	defer server.Close()

	settings := testOpenAISettings(t, server.URL, "key", "gpt-image-1")
	settings.OpenAI.TextModel = "gpt-4o-mini"
	_, err := GenerateText(settings, TextRequest{User: "写一条提示词"})
	if !apperr.Is(err, "PROVIDER_ERROR") {
		t.Fatalf("err = %v", err)
	}
	var target *apperr.Error
	if !errors.As(err, &target) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(target.Detail, "gpt-4o-mini") || !strings.Contains(target.Detail, "文本模型") {
		t.Fatalf("detail = %q", target.Detail)
	}
}

func TestGenerateTextExplainsReasoningOnlyReply(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"choices":[{"finish_reason":"length","message":{"content":null,"reasoning_content":"先分析设定，再决定怎么写……"}}]}`)
	}))
	defer server.Close()

	settings := testOpenAISettings(t, server.URL, "key", "gpt-image-1")
	_, err := GenerateText(settings, TextRequest{User: "写一条提示词", MaxTokens: 400})
	if !apperr.Is(err, "PROVIDER_ERROR") {
		t.Fatalf("err = %v", err)
	}
	var target *apperr.Error
	if !errors.As(err, &target) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(target.Message, "推理") || !strings.Contains(target.Detail, "对话模型") {
		t.Fatalf("message = %q detail = %q", target.Message, target.Detail)
	}
}

func TestGenerateTextUsesOwnTextEndpoint(t *testing.T) {
	var auth, path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, path = r.Header.Get("Authorization"), r.URL.Path
		io.WriteString(w, `{"choices":[{"message":{"content":"独立接口"}}]}`)
	}))
	defer server.Close()

	settings := testOpenAISettings(t, "http://127.0.0.1:1/v1", "image-key", "img")
	settings.OpenAI.TextProvider = TextProviderOwn
	settings.OpenAI.TextBaseURL = server.URL
	settings.OpenAI.TextModel = "chat-model"
	enc, err := secure.Protect("text-key")
	if err != nil {
		t.Fatal(err)
	}
	settings.OpenAI.TextAPIKeyEnc = enc

	got, err := GenerateText(settings, TextRequest{User: "写一段"})
	if err != nil {
		t.Fatalf("GenerateText: %v", err)
	}
	if got.Text != "独立接口" {
		t.Fatalf("text = %q", got.Text)
	}
	if path != "/chat/completions" || auth != "Bearer text-key" {
		t.Fatalf("path=%q auth=%q", path, auth)
	}
}

func TestGenerateTextRequiresOwnTextConfig(t *testing.T) {
	settings := testOpenAISettings(t, "https://api.example.com/v1", "image-key", "img")
	settings.OpenAI.TextProvider = TextProviderOwn
	_, err := GenerateText(settings, TextRequest{User: "x"})
	if !apperr.Is(err, "PROVIDER_AUTH") {
		t.Fatalf("missing base err = %v", err)
	}

	settings.OpenAI.TextBaseURL = "https://text.example.com/v1"
	_, err = GenerateText(settings, TextRequest{User: "x"})
	if !apperr.Is(err, "PROVIDER_AUTH") {
		t.Fatalf("missing key err = %v", err)
	}
}

func TestGenerateTextRequiresKeyAndUser(t *testing.T) {
	settings := testOpenAISettings(t, "https://api.example.com/v1", "", "gpt-image-1")
	if _, err := GenerateText(settings, TextRequest{User: "x"}); !apperr.Is(err, "PROVIDER_AUTH") {
		t.Fatalf("no key err = %v", err)
	}

	settings = testOpenAISettings(t, "https://api.example.com/v1", "key", "gpt-image-1")
	if _, err := GenerateText(settings, TextRequest{User: "   "}); !apperr.Is(err, "INVALID_TEXT") {
		t.Fatalf("empty user err = %v", err)
	}
}
