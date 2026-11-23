package generate

import (
	"os"
	"path/filepath"
	"strings"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
	"pixelbench/internal/project"
	"pixelbench/internal/secure"
)

// Provider identifiers.
const (
	ProviderComfyUI = "comfyui"
	ProviderOpenAI  = "openai"

	// TextProviderShared reuses the image endpoint and key for chat requests.
	TextProviderShared = "shared"
	// TextProviderOwn uses a dedicated base URL and key for chat requests.
	TextProviderOwn = "own"
)

// ComfyConfig is the editable ComfyUI connection and workflow template.
type ComfyConfig struct {
	BaseURL      string `json:"baseUrl"`
	WorkflowName string `json:"workflowName"`
	Workflow     string `json:"workflow"`
}

// OpenAIPublic is the OpenAI-compatible view shown in the UI; it never carries
// the key itself.
type OpenAIPublic struct {
	BaseURL string `json:"baseUrl"`
	Model   string `json:"model"`
	// Optional model for masked edits; empty reuses Model.
	EditModel    string `json:"editModel"`
	TextModel    string `json:"textModel"`
	Size         string `json:"size"`
	HasKey       bool   `json:"hasKey"`
	TextProvider string `json:"textProvider"`
	TextBaseURL  string `json:"textBaseUrl"`
	TextHasKey   bool   `json:"textHasKey"`
}

// SettingsView is the masked settings payload sent to the frontend.
type SettingsView struct {
	DefaultProvider string       `json:"defaultProvider"`
	Comfy           ComfyConfig  `json:"comfy"`
	OpenAI          OpenAIPublic `json:"openai"`
}

// OpenAIInput is what the settings form submits. A blank APIKey keeps the
// stored key; ClearKey removes it.
type OpenAIInput struct {
	BaseURL   string `json:"baseUrl"`
	Model     string `json:"model"`
	EditModel string `json:"editModel,omitempty"`
	TextModel string `json:"textModel"`
	Size      string `json:"size"`
	APIKey    string `json:"apiKey"`
	ClearKey  bool   `json:"clearKey"`
	// TextProvider is "shared" or "own"; empty keeps the stored mode.
	TextProvider string `json:"textProvider,omitempty"`
	TextBaseURL  string `json:"textBaseUrl,omitempty"`
	TextAPIKey   string `json:"textApiKey,omitempty"`
	TextClearKey bool   `json:"textClearKey,omitempty"`
}

// SettingsInput is the full settings form payload.
type SettingsInput struct {
	DefaultProvider string      `json:"defaultProvider"`
	Comfy           ComfyConfig `json:"comfy"`
	OpenAI          OpenAIInput `json:"openai"`
}

type storedOpenAI struct {
	BaseURL   string `json:"baseUrl"`
	Model     string `json:"model"`
	EditModel string `json:"editModel,omitempty"`
	TextModel string `json:"textModel,omitempty"`
	Size      string `json:"size"`
	APIKeyEnc string `json:"apiKeyEnc,omitempty"`
	// TextProvider, TextBaseURL and TextAPIKeyEnc let the text model live on a
	// different vendor than the image model. Empty mode means legacy "shared".
	TextProvider  string `json:"textProvider,omitempty"`
	TextBaseURL   string `json:"textBaseUrl,omitempty"`
	TextAPIKeyEnc string `json:"textApiKeyEnc,omitempty"`
}

type storedSettings struct {
	DefaultProvider string       `json:"defaultProvider"`
	Comfy           ComfyConfig  `json:"comfy"`
	OpenAI          storedOpenAI `json:"openai"`
}

// Settings is the resolved runtime configuration, including the decrypted key
// holder. It stays inside the Go process.
type Settings struct {
	DefaultProvider string
	Comfy           ComfyConfig
	OpenAI          storedOpenAI
}

// APIKey decrypts the OpenAI key. An empty string means "not configured".
func (s Settings) APIKey() (string, error) {
	if s.OpenAI.APIKeyEnc == "" {
		return "", nil
	}
	key, err := secure.Unprotect(s.OpenAI.APIKeyEnc)
	if err != nil {
		return "", apperr.Wrap("PROVIDER_AUTH", "无法读取已保存的 API Key，请重新填写", err)
	}
	return key, nil
}

// TextProviderMode reports whether chat requests reuse the image endpoint.
func (s Settings) TextProviderMode() string {
	if strings.EqualFold(strings.TrimSpace(s.OpenAI.TextProvider), TextProviderOwn) {
		return TextProviderOwn
	}
	return TextProviderShared
}

// TextBaseURL is the endpoint used for chat completions.
func (s Settings) TextBaseURL() string {
	if s.TextProviderMode() == TextProviderOwn {
		return strings.TrimSpace(s.OpenAI.TextBaseURL)
	}
	return strings.TrimSpace(s.OpenAI.BaseURL)
}

// TextKey decrypts the key used for chat completions; own mode never falls
// back to the image key.
func (s Settings) TextKey() (string, error) {
	if s.TextProviderMode() != TextProviderOwn {
		return s.APIKey()
	}
	if s.OpenAI.TextAPIKeyEnc == "" {
		return "", nil
	}
	key, err := secure.Unprotect(s.OpenAI.TextAPIKeyEnc)
	if err != nil {
		return "", apperr.Wrap("PROVIDER_AUTH", "无法读取已保存的文本接口 API Key，请重新填写", err)
	}
	return key, nil
}

// SettingsPath is the app-level AI configuration file.
func SettingsPath() (string, error) {
	dir, err := project.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ai.json"), nil
}

// DefaultWorkflow is a marker-annotated ComfyUI API workflow. Node titles
// PROMPT / NEGATIVE / SEED / SIZE tell the backend which inputs to rewrite.
const DefaultWorkflow = `{
  "3": {
    "class_type": "KSampler",
    "_meta": { "title": "SEED" },
    "inputs": {
      "seed": 42, "steps": 24, "cfg": 7, "sampler_name": "euler",
      "scheduler": "normal", "denoise": 1,
      "model": ["4", 0], "positive": ["6", 0], "negative": ["7", 0], "latent_image": ["5", 0]
    }
  },
  "4": {
    "class_type": "CheckpointLoaderSimple",
    "inputs": { "ckpt_name": "sd_xl_base_1.0.safetensors" }
  },
  "5": {
    "class_type": "EmptyLatentImage",
    "_meta": { "title": "SIZE" },
    "inputs": { "width": 1024, "height": 1024, "batch_size": 1 }
  },
  "6": {
    "class_type": "CLIPTextEncode",
    "_meta": { "title": "PROMPT" },
    "inputs": { "text": "", "clip": ["4", 1] }
  },
  "7": {
    "class_type": "CLIPTextEncode",
    "_meta": { "title": "NEGATIVE" },
    "inputs": { "text": "", "clip": ["4", 1] }
  },
  "8": {
    "class_type": "VAEDecode",
    "inputs": { "samples": ["3", 0], "vae": ["4", 2] }
  },
  "9": {
    "class_type": "SaveImage",
    "inputs": { "filename_prefix": "pixelbench", "images": ["8", 0] }
  }
}`

func defaults() Settings {
	return Settings{
		DefaultProvider: ProviderComfyUI,
		Comfy: ComfyConfig{
			BaseURL:      "http://127.0.0.1:8188",
			WorkflowName: "默认文生图",
			Workflow:     DefaultWorkflow,
		},
		OpenAI: storedOpenAI{
			BaseURL:      "https://api.openai.com/v1",
			Model:        "gpt-image-1",
			TextModel:    "gpt-4o-mini",
			Size:         "1024x1024",
			TextProvider: TextProviderShared,
		},
	}
}

// LoadSettings reads ai.json, filling defaults for anything absent.
func LoadSettings() Settings {
	out := defaults()
	path, err := SettingsPath()
	if err != nil {
		return out
	}
	var stored storedSettings
	if err := fsutil.ReadJSON(path, &stored); err != nil {
		return out
	}
	if stored.DefaultProvider == ProviderComfyUI || stored.DefaultProvider == ProviderOpenAI {
		out.DefaultProvider = stored.DefaultProvider
	}
	if strings.TrimSpace(stored.Comfy.BaseURL) != "" {
		out.Comfy.BaseURL = strings.TrimSpace(stored.Comfy.BaseURL)
	}
	if strings.TrimSpace(stored.Comfy.Workflow) != "" {
		out.Comfy.Workflow = stored.Comfy.Workflow
	}
	if strings.TrimSpace(stored.Comfy.WorkflowName) != "" {
		out.Comfy.WorkflowName = strings.TrimSpace(stored.Comfy.WorkflowName)
	}
	if strings.TrimSpace(stored.OpenAI.BaseURL) != "" {
		out.OpenAI.BaseURL = strings.TrimSpace(stored.OpenAI.BaseURL)
	}
	if strings.TrimSpace(stored.OpenAI.Model) != "" {
		out.OpenAI.Model = strings.TrimSpace(stored.OpenAI.Model)
	}
	out.OpenAI.EditModel = strings.TrimSpace(stored.OpenAI.EditModel)
	if strings.TrimSpace(stored.OpenAI.TextModel) != "" {
		out.OpenAI.TextModel = strings.TrimSpace(stored.OpenAI.TextModel)
	}
	if strings.TrimSpace(stored.OpenAI.Size) != "" {
		out.OpenAI.Size = strings.TrimSpace(stored.OpenAI.Size)
	}
	out.OpenAI.APIKeyEnc = stored.OpenAI.APIKeyEnc
	out.OpenAI.TextProvider = stored.OpenAI.TextProvider
	if out.OpenAI.TextProvider == "" {
		out.OpenAI.TextProvider = TextProviderShared
	}
	out.OpenAI.TextBaseURL = strings.TrimSpace(stored.OpenAI.TextBaseURL)
	out.OpenAI.TextAPIKeyEnc = stored.OpenAI.TextAPIKeyEnc
	return out
}

// View masks the settings for the UI.
func View(settings Settings) SettingsView {
	return SettingsView{
		DefaultProvider: settings.DefaultProvider,
		Comfy:           settings.Comfy,
		OpenAI: OpenAIPublic{
			BaseURL:      settings.OpenAI.BaseURL,
			Model:        settings.OpenAI.Model,
			EditModel:    settings.OpenAI.EditModel,
			TextModel:    settings.OpenAI.TextModel,
			Size:         settings.OpenAI.Size,
			HasKey:       settings.OpenAI.APIKeyEnc != "",
			TextProvider: settings.TextProviderMode(),
			TextBaseURL:  settings.OpenAI.TextBaseURL,
			TextHasKey:   settings.OpenAI.TextAPIKeyEnc != "",
		},
	}
}

// SaveSettings writes the form back, encrypting a newly supplied key.
func SaveSettings(input SettingsInput) (SettingsView, error) {
	current := LoadSettings()
	next := current
	if input.DefaultProvider == ProviderComfyUI || input.DefaultProvider == ProviderOpenAI {
		next.DefaultProvider = input.DefaultProvider
	}
	if base := strings.TrimRight(strings.TrimSpace(input.Comfy.BaseURL), "/"); base != "" {
		next.Comfy.BaseURL = base
	}
	if name := strings.TrimSpace(input.Comfy.WorkflowName); name != "" {
		next.Comfy.WorkflowName = name
	}
	if strings.TrimSpace(input.Comfy.Workflow) != "" {
		next.Comfy.Workflow = input.Comfy.Workflow
	}
	if base := strings.TrimRight(strings.TrimSpace(input.OpenAI.BaseURL), "/"); base != "" {
		next.OpenAI.BaseURL = base
	}
	if model := strings.TrimSpace(input.OpenAI.Model); model != "" {
		next.OpenAI.Model = model
	}
	next.OpenAI.EditModel = strings.TrimSpace(input.OpenAI.EditModel)
	if textModel := strings.TrimSpace(input.OpenAI.TextModel); textModel != "" {
		next.OpenAI.TextModel = textModel
	}
	if size := strings.TrimSpace(input.OpenAI.Size); size != "" {
		next.OpenAI.Size = size
	}
	switch strings.ToLower(strings.TrimSpace(input.OpenAI.TextProvider)) {
	case TextProviderOwn:
		next.OpenAI.TextProvider = TextProviderOwn
	case TextProviderShared:
		next.OpenAI.TextProvider = TextProviderShared
	default:
		if next.OpenAI.TextProvider == "" {
			next.OpenAI.TextProvider = TextProviderShared
		}
	}
	if next.OpenAI.TextProvider == TextProviderOwn {
		next.OpenAI.TextBaseURL = strings.TrimRight(strings.TrimSpace(input.OpenAI.TextBaseURL), "/")
	}
	switch {
	case input.OpenAI.ClearKey:
		next.OpenAI.APIKeyEnc = ""
	case strings.TrimSpace(input.OpenAI.APIKey) != "":
		enc, err := secure.Protect(strings.TrimSpace(input.OpenAI.APIKey))
		if err != nil {
			return SettingsView{}, apperr.Wrap("PROVIDER_AUTH", "无法加密 API Key", err)
		}
		next.OpenAI.APIKeyEnc = enc
	}
	switch {
	case input.OpenAI.TextClearKey:
		next.OpenAI.TextAPIKeyEnc = ""
	case strings.TrimSpace(input.OpenAI.TextAPIKey) != "":
		enc, err := secure.Protect(strings.TrimSpace(input.OpenAI.TextAPIKey))
		if err != nil {
			return SettingsView{}, apperr.Wrap("PROVIDER_AUTH", "无法加密文本接口的 API Key", err)
		}
		next.OpenAI.TextAPIKeyEnc = enc
	}

	path, err := SettingsPath()
	if err != nil {
		return SettingsView{}, apperr.Wrap("IO", "无法定位应用配置目录", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return SettingsView{}, apperr.Wrap("IO", "无法创建应用配置目录", err)
	}
	stored := storedSettings{
		DefaultProvider: next.DefaultProvider,
		Comfy:           next.Comfy,
		OpenAI:          next.OpenAI,
	}
	if err := fsutil.WriteJSONAtomic(path, stored, 0o600); err != nil {
		return SettingsView{}, apperr.Wrap("IO", "无法保存 AI 配置", err)
	}
	return View(next), nil
}
