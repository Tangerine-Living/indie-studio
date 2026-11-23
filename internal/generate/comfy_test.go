package generate

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRewriteWorkflowByMarkers(t *testing.T) {
	req, err := (JobRequest{
		Prompt:         "  一间杂货铺  ",
		NegativePrompt: "模糊",
		Width:          320,
		Height:         240,
		Count:          2,
		Seed:           12345,
	}).normalize(ProviderComfyUI)
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := rewriteWorkflow(DefaultWorkflow, req)
	if err != nil {
		t.Fatal(err)
	}
	var graph map[string]map[string]any
	if err := json.Unmarshal(workflow, &graph); err != nil {
		t.Fatal(err)
	}
	prompt := graph["6"]["inputs"].(map[string]any)
	if prompt["text"] != "一间杂货铺" {
		t.Fatalf("prompt text = %v", prompt["text"])
	}
	negative := graph["7"]["inputs"].(map[string]any)
	if negative["text"] != "模糊" {
		t.Fatalf("negative text = %v", negative["text"])
	}
	latent := graph["5"]["inputs"].(map[string]any)
	if latent["width"] != float64(320) || latent["height"] != float64(240) || latent["batch_size"] != float64(2) {
		t.Fatalf("latent = %v", latent)
	}
	sampler := graph["3"]["inputs"].(map[string]any)
	if sampler["seed"] != float64(12345) {
		t.Fatalf("seed = %v", sampler["seed"])
	}
}

func TestRewriteWorkflowByClasses(t *testing.T) {
	raw := `{
	  "1": {"class_type": "CheckpointLoaderSimple", "inputs": {"ckpt_name": "x"}},
	  "2": {"class_type": "CLIPTextEncode", "inputs": {"text": "old", "clip": ["1", 1]}},
	  "3": {"class_type": "CLIPTextEncode", "inputs": {"text": "", "clip": ["1", 1]}},
	  "4": {"class_type": "EmptyLatentImage", "inputs": {"width": 512, "height": 512, "batch_size": 1}},
	  "5": {"class_type": "KSampler", "inputs": {"seed": 1, "model": ["1", 0]}}
	}`
	req, _ := (JobRequest{Prompt: "塔楼", NegativePrompt: "低清", Width: 256, Height: 512, Count: 1, Seed: 7}).normalize(ProviderComfyUI)
	out, err := rewriteWorkflow(raw, req)
	if err != nil {
		t.Fatal(err)
	}
	var graph map[string]map[string]any
	if err := json.Unmarshal(out, &graph); err != nil {
		t.Fatal(err)
	}
	if graph["2"]["inputs"].(map[string]any)["text"] != "塔楼" {
		t.Fatal("positive prompt not rewritten")
	}
	if graph["3"]["inputs"].(map[string]any)["text"] != "低清" {
		t.Fatal("negative prompt not rewritten")
	}
	if graph["4"]["inputs"].(map[string]any)["width"] != float64(256) {
		t.Fatal("width not rewritten")
	}
	if graph["5"]["inputs"].(map[string]any)["seed"] != float64(7) {
		t.Fatal("seed not rewritten")
	}
}

func TestRewriteWorkflowMissingPromptNode(t *testing.T) {
	raw := `{"1": {"class_type": "KSampler", "inputs": {"seed": 1}}}`
	_, err := rewriteWorkflow(raw, JobRequest{Prompt: "x"})
	if err == nil {
		t.Fatal("expected a workflow error")
	}
}

func TestComfyGenerateOverHTTP(t *testing.T) {
	pngBytes := tinyPNG(t)
	var sawPrompt bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/prompt" && r.Method == http.MethodPost:
			var payload struct {
				Prompt map[string]struct {
					Inputs map[string]any `json:"inputs"`
				} `json:"prompt"`
			}
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Errorf("bad payload: %v", err)
			}
			if payload.Prompt["6"].Inputs["text"] != "a shop" {
				t.Errorf("prompt = %v", payload.Prompt["6"].Inputs["text"])
			}
			sawPrompt = true
			io.WriteString(w, `{"prompt_id":"p1"}`)
		case strings.HasPrefix(r.URL.Path, "/history/"):
			io.WriteString(w, `{"p1":{"status":{"status_str":"success","completed":true},"outputs":{"9":{"images":[{"filename":"a.png","subfolder":"","type":"output"}]}}}}`)
		case r.URL.Path == "/view":
			w.Header().Set("Content-Type", "image/png")
			w.Write(pngBytes)
		case r.URL.Path == "/system_stats":
			io.WriteString(w, `{"system":{"comfyui_version":"0.3.0"},"devices":[{"name":"RTX"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider, err := newComfy(ComfyConfig{BaseURL: server.URL, Workflow: DefaultWorkflow})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	result, err := provider.Test(ctx)
	if err != nil || !result.OK {
		t.Fatalf("test = %+v, %v", result, err)
	}
	if !strings.Contains(result.Detail, "RTX") {
		t.Fatalf("detail = %q", result.Detail)
	}

	images, err := provider.Generate(ctx, JobRequest{
		Prompt: "a shop",
		Width:  512,
		Height: 512,
		Count:  1,
		Seed:   42,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !sawPrompt {
		t.Fatal("prompt endpoint was not called")
	}
	if len(images) != 1 || !bytes.Equal(images[0].Data, pngBytes) {
		t.Fatalf("images = %d", len(images))
	}
}
