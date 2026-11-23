package generate

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pixelbench/internal/apperr"
)

func testMaskPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	img.SetNRGBA(1, 0, color.NRGBA{R: 0, G: 0, B: 0, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func asNRGBA(img image.Image) *image.NRGBA {
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	return out
}

func dataURL(raw []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
}

func TestDecodeImageDataURL(t *testing.T) {
	raw := testMaskPNG(t)
	if out, err := decodeImageDataURL(dataURL(raw)); err != nil || !bytes.Equal(out, raw) {
		t.Fatalf("decode data url: %v", err)
	}
	if out, err := decodeImageDataURL(base64.StdEncoding.EncodeToString(raw)); err != nil || !bytes.Equal(out, raw) {
		t.Fatalf("decode bare base64: %v", err)
	}
	if _, err := decodeImageDataURL("   "); !apperr.Is(err, "INVALID_INPAINT") {
		t.Fatalf("empty error = %v", err)
	}
}

func TestInpaintUsesDedicatedEditModel(t *testing.T) {
	var model string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/edits" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseMultipartForm(8 << 20); err == nil {
			model = r.FormValue("model")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(tinyPNG(t))}},
		})
	}))
	defer server.Close()

	settings := testOpenAISettings(t, server.URL, "key", "generate-model")
	settings.OpenAI.EditModel = "edit-model"
	provider, err := newOpenAI(settings)
	if err != nil {
		t.Fatal(err)
	}
	images, err := provider.inpaint(context.Background(), JobRequest{
		Prompt:    "把手改成握着灯笼",
		Width:     64,
		Height:    64,
		Count:     1,
		BaseImage: dataURL(tinyPNG(t)),
		Mask:      dataURL(testMaskPNG(t)),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 1 || model != "edit-model" {
		t.Fatalf("images = %d model = %q", len(images), model)
	}
}

func TestOpenAIExplainsUnsupportedMaskEdits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/edits" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":{"message":"MiniMax Image does not support mask edits"}}`)
	}))
	defer server.Close()

	settings := testOpenAISettings(t, server.URL, "key", "minimax-image")
	provider, err := newOpenAI(settings)
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.inpaint(context.Background(), JobRequest{
		Prompt:    "把手改成握着灯笼",
		Width:     64,
		Height:    64,
		Count:     1,
		BaseImage: dataURL(tinyPNG(t)),
		Mask:      dataURL(testMaskPNG(t)),
	}, nil)
	if !apperr.Is(err, "PROVIDER_EDITS_UNSUPPORTED") {
		t.Fatalf("err = %v", err)
	}
	var target *apperr.Error
	if !errors.As(err, &target) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(target.Detail, "minimax-image") || !strings.Contains(target.Detail, "ComfyUI") {
		t.Fatalf("detail = %q", target.Detail)
	}
}

func TestMaskConventions(t *testing.T) {
	raw := testMaskPNG(t)
	openAI, err := maskForOpenAI(raw)
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(bytes.NewReader(openAI))
	if err != nil {
		t.Fatal(err)
	}
	nrgba := asNRGBA(img)
	// Painted white becomes fully transparent (edited); black stays opaque.
	if got := nrgba.NRGBAAt(0, 0); got.A != 0 {
		t.Fatalf("openai painted alpha = %d, want 0", got.A)
	}
	if got := nrgba.NRGBAAt(1, 0); got.A != 255 {
		t.Fatalf("openai untouched alpha = %d, want 255", got.A)
	}

	comfy, err := maskForComfy(raw)
	if err != nil {
		t.Fatal(err)
	}
	cimg, _, err := image.Decode(bytes.NewReader(comfy))
	if err != nil {
		t.Fatal(err)
	}
	cn := asNRGBA(cimg)
	if cn.NRGBAAt(0, 0).R != 255 || cn.NRGBAAt(0, 0).A != 255 {
		t.Fatalf("comfy painted = %+v", cn.NRGBAAt(0, 0))
	}
	if cn.NRGBAAt(1, 0).R != 0 || cn.NRGBAAt(1, 0).A != 0 {
		t.Fatalf("comfy untouched = %+v", cn.NRGBAAt(1, 0))
	}
}

func TestOpenAIInpaintPostsMultipart(t *testing.T) {
	pngBytes := tinyPNG(t)
	maskBytes := testMaskPNG(t)
	var seen struct {
		Model, Prompt, Size, N string
		ImageLen, MaskLen      int
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/edits" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			t.Errorf("multipart: %v", err)
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		seen.Model = r.FormValue("model")
		seen.Prompt = r.FormValue("prompt")
		seen.Size = r.FormValue("size")
		seen.N = r.FormValue("n")
		if file, _, err := r.FormFile("image"); err == nil {
			data, _ := io.ReadAll(file)
			seen.ImageLen = len(data)
		}
		if file, _, err := r.FormFile("mask"); err == nil {
			data, _ := io.ReadAll(file)
			seen.MaskLen = len(data)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(pngBytes)}},
		})
	}))
	defer server.Close()

	provider, err := newOpenAI(testOpenAISettings(t, server.URL, "test-key", "gpt-image-1"))
	if err != nil {
		t.Fatal(err)
	}
	images, err := provider.Generate(context.Background(), JobRequest{
		Mode:      JobModeInpaint,
		Prompt:    "把这只手修好",
		Width:     1024,
		Height:    1024,
		Count:     1,
		BaseImage: dataURL(pngBytes),
		Mask:      dataURL(maskBytes),
	}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(images) != 1 || len(images[0].Data) == 0 {
		t.Fatalf("images = %+v", images)
	}
	if seen.Model != "gpt-image-1" || seen.Prompt != "把这只手修好" || seen.N != "1" {
		t.Fatalf("form = %+v", seen)
	}
	if seen.Size != "1024x1024" {
		t.Fatalf("size = %q", seen.Size)
	}
	if seen.ImageLen == 0 || seen.MaskLen == 0 {
		t.Fatalf("missing file parts: image=%d mask=%d", seen.ImageLen, seen.MaskLen)
	}
}

func TestComfyInpaintWorkflowSubstitution(t *testing.T) {
	workflow := `{
	  "1":{"class_type":"LoadImage","_meta":{"title":"IMAGE"},"inputs":{"image":"old.png"}},
	  "2":{"class_type":"LoadImageMask","_meta":{"title":"MASK"},"inputs":{"image":"old-mask.png","channel":"red"}},
	  "3":{"class_type":"CLIPTextEncode","_meta":{"title":"PROMPT"},"inputs":{"text":""}}
	}`
	out, err := rewriteWorkflow(workflow, JobRequest{
		Mode:      JobModeInpaint,
		Prompt:    "修手",
		ImageName: "base-1.png",
		MaskName:  "mask-1.png",
	})
	if err != nil {
		t.Fatalf("rewriteWorkflow: %v", err)
	}
	var graph map[string]struct {
		Inputs map[string]any `json:"inputs"`
	}
	if err := json.Unmarshal(out, &graph); err != nil {
		t.Fatal(err)
	}
	if graph["1"].Inputs["image"] != "base-1.png" {
		t.Fatalf("image node = %+v", graph["1"].Inputs)
	}
	if graph["2"].Inputs["image"] != "mask-1.png" {
		t.Fatalf("mask node = %+v", graph["2"].Inputs)
	}
	if graph["3"].Inputs["text"] != "修手" {
		t.Fatalf("prompt = %+v", graph["3"].Inputs)
	}

	if _, err := rewriteWorkflow(`{"1":{"inputs":{"text":""},"_meta":{"title":"PROMPT"}}}`, JobRequest{
		Mode:      JobModeInpaint,
		Prompt:    "x",
		ImageName: "a.png",
		MaskName:  "b.png",
	}); !apperr.Is(err, "PROVIDER_WORKFLOW") {
		t.Fatalf("missing mask error = %v", err)
	}
}

func TestComfyUploadImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/upload/image" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if r.FormValue("type") != "input" {
			t.Errorf("type field = %q", r.FormValue("type"))
		}
		file, header, err := r.FormFile("image")
		if err != nil {
			http.Error(w, "no file", http.StatusBadRequest)
			return
		}
		_, _ = io.ReadAll(file)
		if header.Filename != "pixelbench-base.png" {
			t.Errorf("filename = %q", header.Filename)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "x.png", "subfolder": "sub"})
	}))
	defer server.Close()

	c := &comfy{base: server.URL, client: httpClient(5 * time.Second)}
	name, err := c.uploadImage(context.Background(), "pixelbench-base.png", []byte{1, 2, 3})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if name != "sub/x.png" {
		t.Fatalf("name = %q", name)
	}
}

func TestInpaintRequestValidation(t *testing.T) {
	if _, err := (JobRequest{Mode: JobModeInpaint, Prompt: "x"}).normalize(ProviderComfyUI); !apperr.Is(err, "INVALID_INPAINT") {
		t.Fatalf("missing masks error = %v", err)
	}
	if _, err := (JobRequest{Mode: "boss", Prompt: "x"}).normalize(ProviderComfyUI); !apperr.Is(err, "INVALID_MODE") {
		t.Fatalf("bad mode error = %v", err)
	}
	out, err := (JobRequest{
		Mode:      JobModeInpaint,
		Prompt:    "x",
		Width:     700,
		Height:    300,
		Count:     4,
		BaseImage: dataURL(testMaskPNG(t)),
		Mask:      dataURL(testMaskPNG(t)),
	}).normalize(ProviderOpenAI)
	if err != nil {
		t.Fatal(err)
	}
	if out.Width%8 != 0 || out.Height%8 != 0 || out.Count != 1 {
		t.Fatalf("normalized = %+v", out)
	}
}
