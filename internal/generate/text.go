package generate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"pixelbench/internal/apperr"
)

// TextRequest is one chat completion: a system role plus the user content.
type TextRequest struct {
	System      string  `json:"system"`
	User        string  `json:"user"`
	MaxTokens   int     `json:"maxTokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

// TextResult is the model's reply, trimmed.
type TextResult struct {
	Text  string `json:"text"`
	Model string `json:"model"`
}

// GenerateText drafts copy or an image prompt through the OpenAI-compatible
// chat endpoint. It uses the same base URL and key as image generation and the
// dedicated text model from settings.
func GenerateText(settings Settings, req TextRequest) (TextResult, error) {
	textBase := settings.TextBaseURL()
	if textBase == "" {
		if settings.TextProviderMode() == TextProviderOwn {
			return TextResult{}, apperr.New("PROVIDER_AUTH", "文本接口是独立配置，但还没填地址").
				WithDetail("到「设置 → AI 生成 → 文本模型」填好文本接口地址，或改成沿用图像接口。")
		}
	}
	base, err := validateHTTPURL(textBase, "文本接口")
	if err != nil {
		return TextResult{}, err
	}
	key, err := settings.TextKey()
	if err != nil {
		return TextResult{}, err
	}
	if strings.TrimSpace(key) == "" {
		if settings.TextProviderMode() == TextProviderOwn {
			return TextResult{}, apperr.New("PROVIDER_AUTH", "文本接口是独立配置，但还没填 API Key").
				WithDetail("到「设置 → AI 生成 → 文本模型」填好文本接口的 API Key，或改成沿用图像接口。")
		}
		return TextResult{}, apperr.New("PROVIDER_AUTH", "写文案和 AI 起稿需要 OpenAI 兼容接口的 API Key，先在「设置 → AI 生成」里填写")
	}
	model := strings.TrimSpace(settings.OpenAI.TextModel)
	if model == "" {
		model = "gpt-4o-mini"
	}
	user := strings.TrimSpace(req.User)
	if user == "" {
		return TextResult{}, apperr.New("INVALID_TEXT", "没有可交给模型的内容")
	}
	temperature := req.Temperature
	if temperature <= 0 {
		temperature = 0.7
	}
	temperature = clampFloat(temperature, 0, 2)
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1200
	}
	maxTokens = clampInt(maxTokens, 128, 4096)

	payload, err := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": strings.TrimSpace(req.System)},
			{"role": "user", "content": user},
		},
		"temperature": temperature,
		"max_tokens":  maxTokens,
	})
	if err != nil {
		return TextResult{}, apperr.Wrap("PROVIDER_ERROR", "无法组装文本请求", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return TextResult{}, apperr.Wrap("PROVIDER_UNAVAILABLE", "接口地址无效", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+key)
	httpReq.Header.Set("Content-Type", "application/json")
	client := httpClient(100 * time.Second)
	resp, err := client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return TextResult{}, ctx.Err()
		}
		return TextResult{}, apperr.Wrap("PROVIDER_UNAVAILABLE", "无法连接文本接口", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return TextResult{}, apperr.Wrap("PROVIDER_ERROR", "读取文本接口响应失败", err)
	}
	if resp.StatusCode >= 400 {
		message, code := parseAPIError(raw)
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return TextResult{}, apperr.New("PROVIDER_AUTH", "API Key 无效或没有权限").WithDetail(message)
		}
		if message == "" {
			message = readSnippet(bytes.NewReader(raw))
		}
		detail := fmt.Sprintf("HTTP %d %s: %s", resp.StatusCode, code, message)
		if isModelNotFound(code, message) {
			detail += fmt.Sprintf("\n文本模型「%s」在这个接口不可用，请到「设置 → AI 生成」改填该接口支持的对话模型。", model)
		}
		return TextResult{}, apperr.New("PROVIDER_ERROR", "文本接口返回错误").WithDetail(detail)
	}
	var response struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content          json.RawMessage `json:"content"`
				ReasoningContent string          `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return TextResult{}, apperr.New("PROVIDER_ERROR", "无法解析文本接口响应").
			WithDetail(readSnippet(bytes.NewReader(raw)))
	}
	if len(response.Choices) == 0 {
		return TextResult{}, apperr.New("PROVIDER_ERROR", "文本接口没有返回内容")
	}
	choice := response.Choices[0]
	content := strings.TrimSpace(decodeTextContent(choice.Message.Content))
	if content == "" {
		// Reasoning models spend the output budget on reasoning_content and can
		// finish without ever emitting the answer.
		if choice.Message.ReasoningContent != "" || choice.FinishReason == "length" {
			return TextResult{}, apperr.New("PROVIDER_ERROR", "模型只返回了推理内容，没有正文").
				WithDetail("当前文本模型是推理模型，输出预算被推理耗尽。请改用更快的对话模型（如 glm-5.3-flash、deepseek-v4-flash），或换一个推理开销更低的模型。")
		}
		return TextResult{}, apperr.New("PROVIDER_ERROR", "文本接口返回了空内容")
	}
	return TextResult{Text: content, Model: model}, nil
}

// decodeTextContent accepts both a plain string and the array-of-parts form
// some gateways use.
func decodeTextContent(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return ""
	}
	var out strings.Builder
	for _, part := range parts {
		if part.Text != "" {
			out.WriteString(part.Text)
		}
	}
	return out.String()
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
