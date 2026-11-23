package generate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"pixelbench/internal/apperr"
)

type comfy struct {
	base     string
	workflow string
	client   *http.Client
}

func newComfy(cfg ComfyConfig) (*comfy, error) {
	base, err := validateHTTPURL(cfg.BaseURL, "ComfyUI")
	if err != nil {
		return nil, err
	}
	workflow := strings.TrimSpace(cfg.Workflow)
	if workflow == "" {
		workflow = DefaultWorkflow
	}
	return &comfy{base: base, workflow: workflow, client: httpClient(60 * time.Second)}, nil
}

func (c *comfy) ID() string { return ProviderComfyUI }

func (c *comfy) Test(ctx context.Context) (TestResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/system_stats", nil)
	if err != nil {
		return TestResult{}, apperr.Wrap("PROVIDER_UNAVAILABLE", "ComfyUI 地址无效", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return TestResult{}, apperr.Wrap("PROVIDER_UNAVAILABLE", "无法连接 ComfyUI，请确认它已启动并监听 "+c.base, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return TestResult{}, apperr.New("PROVIDER_UNAVAILABLE", "ComfyUI 返回了错误状态").
			WithDetail(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, readSnippet(resp.Body)))
	}
	var stats struct {
		System struct {
			Version string `json:"comfyui_version"`
			Python  string `json:"python_version"`
		} `json:"system"`
		Devices []struct {
			Name string `json:"name"`
		} `json:"devices"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&stats)
	detail := stats.System.Version
	if len(stats.Devices) > 0 {
		detail += " · " + stats.Devices[0].Name
	}
	return TestResult{OK: true, Message: "ComfyUI 已连接", Detail: strings.TrimSpace(detail)}, nil
}

type comfyImage struct {
	Filename  string `json:"filename"`
	Subfolder string `json:"subfolder"`
	Type      string `json:"type"`
}

type comfyHistoryEntry struct {
	Status struct {
		StatusStr string `json:"status_str"`
		Completed bool   `json:"completed"`
	} `json:"status"`
	Outputs map[string]struct {
		Images []comfyImage `json:"images"`
	} `json:"outputs"`
}

func (c *comfy) Generate(
	ctx context.Context,
	req JobRequest,
	progress ProgressFunc,
) ([]GeneratedImage, error) {
	report := func(percent int, stage string) {
		if progress != nil {
			progress(percent, stage)
		}
	}
	if req.Mode == JobModeInpaint {
		uploaded, err := c.uploadInpaintImages(ctx, req)
		if err != nil {
			return nil, err
		}
		req.ImageName = uploaded.image
		req.MaskName = uploaded.mask
		report(8, "已上传原图与蒙版")
	}
	workflow, err := rewriteWorkflow(c.workflow, req)
	if err != nil {
		return nil, err
	}
	report(10, "提交工作流")
	payload, _ := json.Marshal(map[string]any{
		"prompt":    json.RawMessage(workflow),
		"client_id": "pixelbench",
	})
	body, err := c.request(ctx, http.MethodPost, c.base+"/prompt", payload)
	if err != nil {
		return nil, err
	}
	var queued struct {
		PromptID string         `json:"prompt_id"`
		NodeErrs map[string]any `json:"node_errors"`
	}
	if err := json.Unmarshal(body, &queued); err != nil || queued.PromptID == "" {
		return nil, apperr.New("PROVIDER_ERROR", "ComfyUI 没有返回任务 ID").
			WithDetail(readSnippet(bytes.NewReader(body)))
	}
	if len(queued.NodeErrs) > 0 {
		raw, _ := json.Marshal(queued.NodeErrs)
		return nil, apperr.New("PROVIDER_WORKFLOW", "ComfyUI 工作流节点报错").WithDetail(readSnippet(bytes.NewReader(raw)))
	}

	entry, err := c.wait(ctx, queued.PromptID, report)
	if err != nil {
		return nil, err
	}

	images := collectComfyImages(entry)
	if len(images) == 0 {
		return nil, apperr.New("PROVIDER_ERROR", "ComfyUI 完成任务但没有输出图片")
	}
	out := make([]GeneratedImage, 0, len(images))
	for _, image := range images {
		if len(out) >= req.Count {
			break
		}
		data, err := c.download(ctx, image)
		if err != nil {
			return nil, err
		}
		out = append(out, GeneratedImage{Data: data, Format: "png"})
	}
	report(95, "下载结果")
	return out, nil
}

func (c *comfy) request(ctx context.Context, method, target string, body []byte) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, apperr.Wrap("PROVIDER_UNAVAILABLE", "ComfyUI 请求无效", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, apperr.Wrap("PROVIDER_UNAVAILABLE", "无法连接 ComfyUI", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, apperr.Wrap("PROVIDER_ERROR", "读取 ComfyUI 响应失败", err)
	}
	if resp.StatusCode >= 400 {
		code := "PROVIDER_ERROR"
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			code = "PROVIDER_WORKFLOW"
		}
		return nil, apperr.New(code, "ComfyUI 拒绝了请求").
			WithDetail(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, readSnippet(bytes.NewReader(raw))))
	}
	return raw, nil
}

func (c *comfy) wait(ctx context.Context, promptID string, report ProgressFunc) (comfyHistoryEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	started := time.Now()
	target := c.base + "/history/" + url.PathEscape(promptID)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return comfyHistoryEntry{}, apperr.New("PROVIDER_TIMEOUT", "ComfyUI 生成超时（10 分钟）")
			}
			return comfyHistoryEntry{}, ctx.Err()
		case <-ticker.C:
		}
		body, err := c.request(ctx, http.MethodGet, target, nil)
		if err != nil {
			return comfyHistoryEntry{}, err
		}
		var history map[string]comfyHistoryEntry
		if err := json.Unmarshal(body, &history); err != nil {
			continue
		}
		entry, ok := history[promptID]
		if !ok {
			elapsed := time.Since(started)
			percent := 20 + int(elapsed.Minutes()*6)
			if percent > 85 {
				percent = 85
			}
			report(percent, "ComfyUI 正在生成")
			continue
		}
		if entry.Status.StatusStr == "error" {
			return comfyHistoryEntry{}, apperr.New("PROVIDER_ERROR", "ComfyUI 执行工作流失败")
		}
		if entry.Status.Completed || entry.Status.StatusStr == "success" {
			if len(entry.Outputs) == 0 {
				continue
			}
			return entry, nil
		}
		report(85, "ComfyUI 正在生成")
	}
}

func collectComfyImages(entry comfyHistoryEntry) []comfyImage {
	nodes := make([]string, 0, len(entry.Outputs))
	for id := range entry.Outputs {
		nodes = append(nodes, id)
	}
	sort.Strings(nodes)
	out := []comfyImage{}
	for _, id := range nodes {
		for _, image := range entry.Outputs[id].Images {
			if image.Filename == "" {
				continue
			}
			if image.Type != "" && image.Type != "output" {
				continue
			}
			out = append(out, image)
		}
	}
	return out
}

func (c *comfy) download(ctx context.Context, image comfyImage) ([]byte, error) {
	query := url.Values{}
	query.Set("filename", image.Filename)
	if image.Subfolder != "" {
		query.Set("subfolder", image.Subfolder)
	}
	if image.Type != "" {
		query.Set("type", image.Type)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/view?"+query.Encode(), nil)
	if err != nil {
		return nil, apperr.Wrap("PROVIDER_ERROR", "ComfyUI 图片地址无效", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, apperr.Wrap("PROVIDER_UNAVAILABLE", "无法下载 ComfyUI 结果", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, apperr.New("PROVIDER_ERROR", "无法下载 ComfyUI 结果").
			WithDetail(fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, apperr.Wrap("PROVIDER_ERROR", "读取 ComfyUI 结果失败", err)
	}
	return data, nil
}

// setWorkflowImage fills whichever image field the node uses.
func setWorkflowImage(inputs map[string]any, name string) {
	if _, ok := inputs["image"]; ok {
		inputs["image"] = name
		return
	}
	if _, ok := inputs["filename"]; ok {
		inputs["filename"] = name
		return
	}
	inputs["image"] = name
}

type uploadedInpaint struct {
	image string
	mask  string
}

// uploadInpaintImages sends the base image and mask to ComfyUI's input folder.
func (c *comfy) uploadInpaintImages(ctx context.Context, req JobRequest) (uploadedInpaint, error) {
	base, err := decodeImageDataURL(req.BaseImage)
	if err != nil {
		return uploadedInpaint{}, err
	}
	rawMask, err := decodeImageDataURL(req.Mask)
	if err != nil {
		return uploadedInpaint{}, err
	}
	mask, err := maskForComfy(rawMask)
	if err != nil {
		return uploadedInpaint{}, err
	}
	image, err := c.uploadImage(ctx, "pixelbench-base.png", base)
	if err != nil {
		return uploadedInpaint{}, err
	}
	maskName, err := c.uploadImage(ctx, "pixelbench-mask.png", mask)
	if err != nil {
		return uploadedInpaint{}, err
	}
	return uploadedInpaint{image: image, mask: maskName}, nil
}

func (c *comfy) uploadImage(ctx context.Context, filename string, data []byte) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		return "", apperr.Wrap("PROVIDER_ERROR", "无法组装上传请求", err)
	}
	if _, err := part.Write(data); err != nil {
		return "", apperr.Wrap("PROVIDER_ERROR", "无法组装上传请求", err)
	}
	_ = writer.WriteField("type", "input")
	_ = writer.WriteField("overwrite", "true")
	if err := writer.Close(); err != nil {
		return "", apperr.Wrap("PROVIDER_ERROR", "无法组装上传请求", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/upload/image", &body)
	if err != nil {
		return "", apperr.Wrap("PROVIDER_UNAVAILABLE", "ComfyUI 地址无效", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", apperr.Wrap("PROVIDER_UNAVAILABLE", "无法上传图片到 ComfyUI", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", apperr.Wrap("PROVIDER_ERROR", "读取 ComfyUI 上传响应失败", err)
	}
	if resp.StatusCode >= 400 {
		return "", apperr.New("PROVIDER_WORKFLOW", "ComfyUI 拒绝上传图片").
			WithDetail(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, readSnippet(bytes.NewReader(raw))))
	}
	var out struct {
		Name      string `json:"name"`
		Subfolder string `json:"subfolder"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Name == "" {
		return "", apperr.New("PROVIDER_ERROR", "ComfyUI 没有返回上传文件名").
			WithDetail(readSnippet(bytes.NewReader(raw)))
	}
	if out.Subfolder != "" {
		return out.Subfolder + "/" + out.Name, nil
	}
	return out.Name, nil
}

// rewriteWorkflow injects the request into the workflow. Nodes marked with
// `_meta.title` containing PROMPT / NEGATIVE / SEED / SIZE are rewritten; if no
// marker is found it falls back to the standard SD node classes.
func rewriteWorkflow(raw string, req JobRequest) ([]byte, error) {
	var parsed map[string]any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&parsed); err != nil {
		return nil, apperr.Wrap("PROVIDER_WORKFLOW", "工作流不是有效的 JSON", err)
	}
	graph := parsed
	if inner, ok := parsed["prompt"].(map[string]any); ok {
		graph = inner
	}
	if len(graph) == 0 {
		return nil, apperr.New("PROVIDER_WORKFLOW", "工作流是空的")
	}
	seed := req.Seed
	if seed == 0 {
		seed = time.Now().UnixNano() % 1_000_000_000
		if seed < 0 {
			seed = -seed
		}
	}
	promptSet := false
	imageSet := false
	maskSet := false
	for _, nodeAny := range graph {
		node, ok := nodeAny.(map[string]any)
		if !ok {
			continue
		}
		inputs, ok := node["inputs"].(map[string]any)
		if !ok {
			continue
		}
		title := ""
		if meta, ok := node["_meta"].(map[string]any); ok {
			if value, ok := meta["title"].(string); ok {
				title = strings.ToUpper(value)
			}
		}
		switch {
		case strings.Contains(title, "NEGATIVE"):
			inputs["text"] = req.NegativePrompt
		case strings.Contains(title, "PROMPT"):
			inputs["text"] = req.Prompt
			promptSet = true
		case strings.Contains(title, "SEED"):
			inputs["seed"] = seed
		case strings.Contains(title, "MASK"):
			if req.Mode == JobModeInpaint && req.MaskName != "" {
				setWorkflowImage(inputs, req.MaskName)
				maskSet = true
			}
		case strings.Contains(title, "IMAGE"):
			if req.Mode == JobModeInpaint && req.ImageName != "" {
				setWorkflowImage(inputs, req.ImageName)
				imageSet = true
			}
		case strings.Contains(title, "SIZE"):
			inputs["width"] = req.Width
			inputs["height"] = req.Height
			inputs["batch_size"] = req.Count
		case strings.Contains(title, "WIDTH"):
			inputs["width"] = req.Width
		case strings.Contains(title, "HEIGHT"):
			inputs["height"] = req.Height
		}
	}
	if req.Mode == JobModeInpaint && (!imageSet || !maskSet) {
		return nil, apperr.New("PROVIDER_WORKFLOW", "当前 ComfyUI 工作流不支持局部重绘").
			WithDetail("需要在工作流里准备图像输入节点（标题含 IMAGE）与蒙版节点（标题含 MASK），后端会自动填入上传的原图和蒙版。")
	}
	if !promptSet {
		if err := rewriteByClass(graph, req, seed); err != nil {
			return nil, err
		}
	}
	out, err := json.Marshal(graph)
	if err != nil {
		return nil, apperr.Wrap("PROVIDER_WORKFLOW", "无法序列化工作流", err)
	}
	return out, nil
}

func rewriteByClass(graph map[string]any, req JobRequest, seed int64) error {
	ids := make([]string, 0, len(graph))
	for id := range graph {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	textNodes := []map[string]any{}
	for _, id := range ids {
		node, ok := graph[id].(map[string]any)
		if !ok {
			continue
		}
		class, _ := node["class_type"].(string)
		inputs, _ := node["inputs"].(map[string]any)
		if inputs == nil {
			continue
		}
		switch class {
		case "CLIPTextEncode":
			textNodes = append(textNodes, inputs)
		case "KSampler", "KSamplerAdvanced":
			if _, ok := inputs["seed"]; ok {
				inputs["seed"] = seed
			}
			if _, ok := inputs["noise_seed"]; ok {
				inputs["noise_seed"] = seed
			}
		case "EmptyLatentImage":
			inputs["width"] = req.Width
			inputs["height"] = req.Height
			inputs["batch_size"] = req.Count
		}
	}
	if len(textNodes) == 0 {
		return apperr.New("PROVIDER_WORKFLOW", "工作流里找不到提示词节点").
			WithDetail("给节点加上 _meta.title = PROMPT 即可指定")
	}
	textNodes[0]["text"] = req.Prompt
	if len(textNodes) > 1 && req.NegativePrompt != "" {
		textNodes[1]["text"] = req.NegativePrompt
	}
	return nil
}

var _ Provider = (*comfy)(nil)
