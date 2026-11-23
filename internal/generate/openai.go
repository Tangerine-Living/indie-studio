package generate

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"pixelbench/internal/apperr"
)

type openAI struct {
	base      string
	model     string
	editModel string
	size      string
	key       string
	client    *http.Client
	// Text endpoint resolved from settings; may share base/key with images.
	textBase  string
	textModel string
	textKey   string
	textErr   error
}

func newOpenAI(settings Settings) (*openAI, error) {
	base, err := validateHTTPURL(settings.OpenAI.BaseURL, "OpenAI 兼容接口")
	if err != nil {
		return nil, err
	}
	key, err := settings.APIKey()
	if err != nil {
		return nil, err
	}
	model := strings.TrimSpace(settings.OpenAI.Model)
	if model == "" {
		model = "gpt-image-1"
	}
	editModel := strings.TrimSpace(settings.OpenAI.EditModel)
	if editModel == "" {
		editModel = model
	}
	size := strings.TrimSpace(settings.OpenAI.Size)
	if size == "" {
		size = "1024x1024"
	}
	o := &openAI{
		base:      base,
		model:     model,
		editModel: editModel,
		size:      size,
		key:       key,
		client:    httpClient(10 * time.Minute),
		textModel: strings.TrimSpace(settings.OpenAI.TextModel),
	}
	if o.textModel == "" {
		o.textModel = "gpt-4o-mini"
	}
	o.textBase, o.textKey, o.textErr = settings.TextBaseURL(), "", nil
	if o.textBase != "" {
		o.textBase, o.textErr = validateHTTPURL(o.textBase, "文本接口")
	}
	if settings.TextProviderMode() == TextProviderOwn && o.textBase == "" && o.textErr == nil {
		o.textErr = apperr.New("PROVIDER_AUTH", "文本接口还没填地址")
	}
	if o.textErr == nil {
		o.textKey, o.textErr = settings.TextKey()
	}
	if o.textErr == nil && strings.TrimSpace(o.textKey) == "" {
		o.textErr = apperr.New("PROVIDER_AUTH", "文本接口还没有 API Key")
	}
	return o, nil
}

func (o *openAI) ID() string { return ProviderOpenAI }

func (o *openAI) requireKey() error {
	if strings.TrimSpace(o.key) == "" {
		return apperr.New("PROVIDER_AUTH", "请先在设置里填写 OpenAI 兼容接口的 API Key")
	}
	return nil
}

func (o *openAI) do(ctx context.Context, method, target string, body []byte) ([]byte, error) {
	if err := o.requireKey(); err != nil {
		return nil, err
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, apperr.Wrap("PROVIDER_UNAVAILABLE", "接口地址无效", err)
	}
	req.Header.Set("Authorization", "Bearer "+o.key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return o.send(ctx, req)
}

// send runs one prepared request and turns API errors into readable errors.
func (o *openAI) send(ctx context.Context, req *http.Request) ([]byte, error) {
	resp, err := o.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, apperr.Wrap("PROVIDER_UNAVAILABLE", "无法连接图像接口", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, apperr.Wrap("PROVIDER_ERROR", "读取接口响应失败", err)
	}
	if resp.StatusCode >= 400 {
		message, code := parseAPIError(raw)
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, apperr.New("PROVIDER_AUTH", "API Key 无效或没有权限").WithDetail(message)
		}
		if message == "" {
			message = readSnippet(bytes.NewReader(raw))
		}
		detail := fmt.Sprintf("HTTP %d %s: %s", resp.StatusCode, code, message)
		lower := strings.ToLower(message)
		if strings.Contains(lower, "mask") && strings.Contains(lower, "support") {
			if o.model != "" {
				detail = fmt.Sprintf("图像模型「%s」不支持蒙版编辑。\n%s", o.model, detail)
			}
			detail += "\n请换用支持 /images/edits 的图像模型（例如 gpt-image 系列），或改用 ComfyUI 局部重绘工作流。"
			return nil, apperr.New("PROVIDER_EDITS_UNSUPPORTED", "当前图像模型不支持蒙版编辑").WithDetail(detail)
		}
		if isModelNotFound(code, message) {
			if ids, listErr := o.modelIDs(ctx); listErr == nil && len(ids) > 0 {
				detail += "\n该账户目前可用的模型：" + sampleModels(ids, 12)
			}
			detail += "\n若模型确实存在，多在模型名后带线路号：把模型写成 模型名@线路号，例如 gemini-4-argon@156。"
			return nil, apperr.New("PROVIDER_MODEL", "该模型在当前分组下没有可用渠道").WithDetail(detail)
		}
		if resp.StatusCode == 404 || resp.StatusCode == 400 || resp.StatusCode == 405 {
			detail += "\n若该服务商不支持 /images/edits，请改用 ComfyUI 局部重绘工作流。"
		}
		return nil, apperr.New("PROVIDER_ERROR", "图像接口返回错误").WithDetail(detail)
	}
	return raw, nil
}

// inpaint sends the base image and mask to the OpenAI-compatible edits API.
func (o *openAI) inpaint(
	ctx context.Context,
	req JobRequest,
	progress ProgressFunc,
) ([]GeneratedImage, error) {
	if err := o.requireKey(); err != nil {
		return nil, err
	}
	base, err := decodeImageDataURL(req.BaseImage)
	if err != nil {
		return nil, err
	}
	rawMask, err := decodeImageDataURL(req.Mask)
	if err != nil {
		return nil, err
	}
	mask, err := maskForOpenAI(rawMask)
	if err != nil {
		return nil, err
	}
	if progress != nil {
		progress(20, "提交局部重绘")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", o.editModel)
	_ = writer.WriteField("prompt", req.Prompt)
	_ = writer.WriteField("size", sizeFor(req.Width, req.Height))
	_ = writer.WriteField("n", "1")
	imagePart, err := writer.CreateFormFile("image", "image.png")
	if err != nil {
		return nil, apperr.Wrap("PROVIDER_ERROR", "无法组装局部重绘请求", err)
	}
	if _, err := imagePart.Write(base); err != nil {
		return nil, apperr.Wrap("PROVIDER_ERROR", "无法组装局部重绘请求", err)
	}
	maskPart, err := writer.CreateFormFile("mask", "mask.png")
	if err != nil {
		return nil, apperr.Wrap("PROVIDER_ERROR", "无法组装局部重绘请求", err)
	}
	if _, err := maskPart.Write(mask); err != nil {
		return nil, apperr.Wrap("PROVIDER_ERROR", "无法组装局部重绘请求", err)
	}
	if err := writer.Close(); err != nil {
		return nil, apperr.Wrap("PROVIDER_ERROR", "无法组装局部重绘请求", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.base+"/images/edits", &body)
	if err != nil {
		return nil, apperr.Wrap("PROVIDER_UNAVAILABLE", "接口地址无效", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+o.key)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	raw, err := o.send(ctx, httpReq)
	if err != nil {
		return nil, err
	}
	if progress != nil {
		progress(85, "下载局部重绘结果")
	}
	return o.decodeImages(ctx, raw)
}

func parseAPIError(raw []byte) (message, code string) {
	var payload struct {
		Error struct {
			Message string `json:"message"`
			Code    any    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", ""
	}
	if payload.Error.Code != nil {
		code = fmt.Sprint(payload.Error.Code)
	}
	return strings.TrimSpace(payload.Error.Message), code
}

func (o *openAI) Test(ctx context.Context) (TestResult, error) {
	if err := o.requireKey(); err != nil {
		return TestResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ids, err := o.modelIDs(ctx)
	if err != nil {
		return TestResult{}, err
	}
	result := TestResult{OK: true, Message: "图像接口已连接", Detail: o.model}
	if len(ids) > 0 && !containsModel(ids, o.model) {
		result = TestResult{
			OK:      false,
			Message: "图像接口可用，但图像模型不在可用列表中",
			Detail:  fmt.Sprintf("当前填写 %s，账户可用：%s", o.model, sampleModels(ids, 12)),
		}
	} else if len(ids) > 0 {
		result.Detail = fmt.Sprintf("%s（账户共 %d 个模型）", o.model, len(ids))
	}
	result.Text = o.testText(ctx, ids)
	return result, nil
}

// testText checks the chat endpoint and the configured text model. It reuses
// the model list when both endpoints share a base URL.
func (o *openAI) testText(ctx context.Context, sharedIDs []string) *TestResult {
	if o.textErr != nil {
		message, detail := "文本接口不可用", ""
		var app *apperr.Error
		if errors.As(o.textErr, &app) {
			message, detail = app.Message, app.Detail
		} else {
			message = o.textErr.Error()
		}
		return &TestResult{OK: false, Message: message, Detail: detail}
	}
	ids := sharedIDs
	if o.textBase != o.base {
		var err error
		ids, err = o.modelIDsAt(ctx, o.textBase, o.textKey)
		if err != nil {
			message, detail := "文本接口不可用", ""
			var app *apperr.Error
			if errors.As(err, &app) {
				message, detail = app.Message, app.Detail
			} else {
				message = err.Error()
			}
			return &TestResult{OK: false, Message: message, Detail: detail}
		}
	}
	detail := o.textModel
	if len(ids) > 0 && !containsModel(ids, o.textModel) {
		return &TestResult{
			OK:      false,
			Message: "文本模型不在该接口的可用列表中",
			Detail:  fmt.Sprintf("当前填写 %s，接口可用：%s", o.textModel, sampleModels(ids, 12)),
		}
	}
	if len(ids) > 0 {
		detail = fmt.Sprintf("%s（接口共 %d 个模型）", o.textModel, len(ids))
	}
	if o.textBase != o.base {
		detail += " · " + o.textBase
	}
	return &TestResult{OK: true, Message: "文本模型已连接", Detail: detail}
}

// modelIDs reads /models for the account's model list. Some proxies omit the
// endpoint or return an empty list; callers treat that as "unknown".
func (o *openAI) modelIDs(ctx context.Context) ([]string, error) {
	return o.modelIDsAt(ctx, o.base, o.key)
}

func (o *openAI) modelIDsAt(ctx context.Context, base, key string) ([]string, error) {
	if strings.TrimSpace(key) == "" {
		return nil, apperr.New("PROVIDER_AUTH", "还没有填写 API Key")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/models", nil)
	if err != nil {
		return nil, apperr.Wrap("PROVIDER_UNAVAILABLE", "接口地址无效", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := o.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, apperr.Wrap("PROVIDER_UNAVAILABLE", "无法连接接口", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, apperr.Wrap("PROVIDER_ERROR", "读取模型列表失败", err)
	}
	if resp.StatusCode >= 400 {
		message, code := parseAPIError(raw)
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, apperr.New("PROVIDER_AUTH", "API Key 无效或没有权限").WithDetail(message)
		}
		if message == "" {
			message = readSnippet(bytes.NewReader(raw))
		}
		return nil, apperr.New("PROVIDER_ERROR", "读取模型列表失败").
			WithDetail(fmt.Sprintf("HTTP %d %s: %s", resp.StatusCode, code, message))
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		// A proxy may answer /models with a non-standard body; treat the list
		// as unknown instead of blocking generation on it.
		return nil, nil
	}
	ids := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		if id := strings.TrimSpace(item.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// isModelNotFound recognises the wording used by OpenAI-compatible proxies
// (NewAPI, one-api) when the model has no channel in the account's group.
func isModelNotFound(code, message string) bool {
	text := strings.ToLower(code + " " + message)
	return strings.Contains(text, "model_not_found") ||
		strings.Contains(text, "no available channel") ||
		strings.Contains(text, "model not found") ||
		strings.Contains(text, "模型不存在") ||
		strings.Contains(text, "无可用渠道") ||
		(strings.Contains(text, "does not exist") && strings.Contains(text, "model"))
}

func containsModel(ids []string, model string) bool {
	for _, id := range ids {
		if strings.EqualFold(id, model) {
			return true
		}
	}
	return false
}

// sampleModels lists image-capable ids first so the suggestion is useful
// rather than a page of chat models.
func sampleModels(ids []string, limit int) string {
	image, other := make([]string, 0, limit), make([]string, 0, limit)
	for _, id := range ids {
		lower := strings.ToLower(id)
		if strings.Contains(lower, "image") || strings.Contains(lower, "dall") ||
			strings.Contains(lower, "flux") || strings.Contains(lower, "stable") ||
			strings.Contains(lower, "seedream") || strings.Contains(lower, "qwen-image") {
			image = append(image, id)
		} else {
			other = append(other, id)
		}
	}
	ordered := append(image, other...)
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}
	return strings.Join(ordered, ", ")
}

// sizeFor maps the requested box onto a model-supported aspect ratio.
func sizeFor(width, height int) string {
	ratio := float64(width) / float64(height)
	switch {
	case ratio >= 1.2:
		return "1536x1024"
	case ratio <= 0.83:
		return "1024x1536"
	default:
		return "1024x1024"
	}
}

func (o *openAI) Generate(
	ctx context.Context,
	req JobRequest,
	progress ProgressFunc,
) ([]GeneratedImage, error) {
	if req.Mode == JobModeInpaint {
		return o.inpaint(ctx, req, progress)
	}
	report := func(percent int, stage string) {
		if progress != nil {
			progress(percent, stage)
		}
	}
	if err := o.requireKey(); err != nil {
		return nil, err
	}
	count := req.Count
	if count < 1 {
		count = 1
	}
	size := sizeFor(req.Width, req.Height)
	out := make([]GeneratedImage, 0, count)
	// Some gateways generate exactly one image per call and reject `n`, so ask
	// once per requested variant instead of relying on the parameter.
	for index := 0; index < count; index++ {
		if count == 1 {
			report(15, "提交图像请求")
		} else {
			report(15+60*index/count, fmt.Sprintf("提交图像请求 %d/%d", index+1, count))
		}
		payload, _ := json.Marshal(o.generationPayload(req.Prompt, size))
		body, err := o.do(ctx, http.MethodPost, o.base+"/images/generations", payload)
		if err != nil {
			return nil, err
		}
		images, err := o.decodeImages(ctx, body)
		if err != nil {
			return nil, err
		}
		out = append(out, images...)
		if len(out) >= count {
			out = out[:count]
			break
		}
	}
	report(85, "下载结果")
	if len(out) == 0 {
		return nil, apperr.New("PROVIDER_ERROR", "图像接口没有返回可用图片")
	}
	return out, nil
}

// generationPayload keeps the request to the widest-common subset. `n` and
// response_format are omitted because gateways disagree on them and both
// default to one base64 image.
func (o *openAI) generationPayload(prompt, size string) map[string]any {
	return map[string]any{
		"model":  o.model,
		"prompt": prompt,
		"size":   size,
	}
}

func (o *openAI) decodeImages(ctx context.Context, body []byte) ([]GeneratedImage, error) {
	var response struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, apperr.New("PROVIDER_ERROR", "无法解析图像接口响应").
			WithDetail(readSnippet(bytes.NewReader(body)))
	}
	if len(response.Data) == 0 {
		return nil, apperr.New("PROVIDER_ERROR", "图像接口没有返回结果")
	}
	out := make([]GeneratedImage, 0, len(response.Data))
	for _, item := range response.Data {
		if item.B64JSON != "" {
			data, err := base64.StdEncoding.DecodeString(item.B64JSON)
			if err != nil {
				return nil, apperr.Wrap("PROVIDER_ERROR", "图像数据损坏", err)
			}
			out = append(out, GeneratedImage{Data: data, Format: "png"})
			continue
		}
		if item.URL != "" {
			dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, item.URL, nil)
			if err != nil {
				return nil, apperr.Wrap("PROVIDER_ERROR", "结果地址无效", err)
			}
			resp, err := o.client.Do(dlReq)
			if err != nil {
				return nil, apperr.Wrap("PROVIDER_UNAVAILABLE", "无法下载生成结果", err)
			}
			data, readErr := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
			resp.Body.Close()
			if readErr != nil {
				return nil, apperr.Wrap("PROVIDER_ERROR", "读取生成结果失败", readErr)
			}
			out = append(out, GeneratedImage{Data: data, Format: "png"})
		}
	}
	return out, nil
}

var _ Provider = (*openAI)(nil)
