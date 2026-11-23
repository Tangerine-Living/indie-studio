package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withTempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PIXELBENCH_CONFIG_DIR", dir)
	return dir
}

func TestSettingsEditModelRoundTrip(t *testing.T) {
	withTempConfig(t)
	if _, err := SaveSettings(SettingsInput{
		DefaultProvider: ProviderOpenAI,
		OpenAI: OpenAIInput{
			BaseURL:   "https://img.example.com/v1",
			Model:     "img-model",
			EditModel: "edit-model",
			TextModel: "chat-model",
			APIKey:    "image-secret",
		},
	}); err != nil {
		t.Fatal(err)
	}
	loaded := LoadSettings()
	if loaded.OpenAI.EditModel != "edit-model" {
		t.Fatalf("edit model = %q", loaded.OpenAI.EditModel)
	}
	if view := View(loaded); view.OpenAI.EditModel != "edit-model" {
		t.Fatalf("view edit model = %q", view.OpenAI.EditModel)
	}

	// Clearing the field falls back to the image model.
	if _, err := SaveSettings(SettingsInput{
		DefaultProvider: ProviderOpenAI,
		OpenAI: OpenAIInput{
			BaseURL:   "https://img.example.com/v1",
			Model:     "img-model",
			EditModel: "",
			TextModel: "chat-model",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if loaded := LoadSettings(); loaded.OpenAI.EditModel != "" {
		t.Fatalf("cleared edit model = %q", loaded.OpenAI.EditModel)
	}
}

func TestSettingsOwnTextProviderRoundTrip(t *testing.T) {
	dir := withTempConfig(t)
	view, err := SaveSettings(SettingsInput{
		DefaultProvider: ProviderOpenAI,
		Comfy:           ComfyConfig{BaseURL: "http://127.0.0.1:9000", WorkflowName: "测试工作流"},
		OpenAI: OpenAIInput{
			BaseURL:      "https://img.example.com/v1",
			Model:        "img-model",
			TextModel:    "chat-model",
			Size:         "1024x1024",
			APIKey:       "image-secret",
			TextProvider: TextProviderOwn,
			TextBaseURL:  "https://text.example.com/v1/",
			TextAPIKey:   "text-secret",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.OpenAI.TextProvider != TextProviderOwn || !view.OpenAI.TextHasKey {
		t.Fatalf("view = %+v", view.OpenAI)
	}
	if view.OpenAI.TextBaseURL != "https://text.example.com/v1" {
		t.Fatalf("text base = %q", view.OpenAI.TextBaseURL)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "ai.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "image-secret") || strings.Contains(string(raw), "text-secret") {
		t.Fatal("a key was written in plain text")
	}

	loaded := LoadSettings()
	if loaded.TextProviderMode() != TextProviderOwn {
		t.Fatalf("mode = %q", loaded.TextProviderMode())
	}
	if loaded.TextBaseURL() != "https://text.example.com/v1" {
		t.Fatalf("resolved text base = %q", loaded.TextBaseURL())
	}
	textKey, err := loaded.TextKey()
	if err != nil || textKey != "text-secret" {
		t.Fatalf("text key = %q err = %v", textKey, err)
	}
	imageKey, err := loaded.APIKey()
	if err != nil || imageKey != "image-secret" {
		t.Fatalf("image key = %q err = %v", imageKey, err)
	}

	// Switching back to shared keeps the image endpoint for chat.
	if _, err := SaveSettings(SettingsInput{
		DefaultProvider: ProviderOpenAI,
		OpenAI: OpenAIInput{
			BaseURL:      "https://img.example.com/v1",
			TextModel:    "chat-model",
			TextProvider: TextProviderShared,
		},
	}); err != nil {
		t.Fatal(err)
	}
	shared := LoadSettings()
	if shared.TextProviderMode() != TextProviderShared || shared.TextBaseURL() != "https://img.example.com/v1" {
		t.Fatalf("shared = %+v", shared.OpenAI)
	}
}

func TestSettingsDefaultsAndKeyRoundTrip(t *testing.T) {
	dir := withTempConfig(t)
	settings := LoadSettings()
	if settings.DefaultProvider != ProviderComfyUI {
		t.Fatalf("default provider = %q", settings.DefaultProvider)
	}
	if settings.Comfy.BaseURL != "http://127.0.0.1:8188" {
		t.Fatalf("default comfy url = %q", settings.Comfy.BaseURL)
	}
	if settings.Comfy.Workflow == "" {
		t.Fatal("default workflow is empty")
	}

	view, err := SaveSettings(SettingsInput{
		DefaultProvider: ProviderOpenAI,
		Comfy:           ComfyConfig{BaseURL: "http://127.0.0.1:9000", WorkflowName: "测试工作流"},
		OpenAI: OpenAIInput{
			BaseURL: "https://api.siliconflow.cn/v1",
			Model:   "qwen-image",
			Size:    "1024x1024",
			APIKey:  "test-key-value-123",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.DefaultProvider != ProviderOpenAI || !view.OpenAI.HasKey {
		t.Fatalf("view = %+v", view)
	}
	if view.OpenAI.Model != "qwen-image" {
		t.Fatalf("model = %q", view.OpenAI.Model)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "ai.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "test-key-value-123") {
		t.Fatal("API key was written in plain text")
	}

	loaded := LoadSettings()
	key, err := loaded.APIKey()
	if err != nil {
		t.Fatal(err)
	}
	if key != "test-key-value-123" {
		t.Fatalf("key round trip = %q", key)
	}

	// A blank key keeps the stored one.
	if _, err := SaveSettings(SettingsInput{
		DefaultProvider: ProviderOpenAI,
		OpenAI:          OpenAIInput{BaseURL: "https://api.siliconflow.cn/v1"},
	}); err != nil {
		t.Fatal(err)
	}
	if key, _ := LoadSettings().APIKey(); key != "test-key-value-123" {
		t.Fatalf("blank key replaced stored key: %q", key)
	}

	// ClearKey removes it.
	if _, err := SaveSettings(SettingsInput{
		DefaultProvider: ProviderOpenAI,
		OpenAI:          OpenAIInput{ClearKey: true},
	}); err != nil {
		t.Fatal(err)
	}
	if key, _ := LoadSettings().APIKey(); key != "" {
		t.Fatalf("key was not cleared: %q", key)
	}
}

func TestSettingsRejectsBadProviderID(t *testing.T) {
	withTempConfig(t)
	if _, err := NewProvider(LoadSettings(), "unknown"); err == nil {
		t.Fatal("expected unknown provider error")
	}
}
