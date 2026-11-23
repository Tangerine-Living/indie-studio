package generate

import (
	"strings"
	"time"

	"pixelbench/internal/apperr"
)

// JobState is the lifecycle of one generation job.
type JobState string

// Job states.
const (
	JobQueued   JobState = "queued"
	JobRunning  JobState = "running"
	JobDone     JobState = "done"
	JobFailed   JobState = "failed"
	JobCanceled JobState = "canceled"
)

// Job modes.
const (
	JobModeGenerate = "generate"
	JobModeInpaint  = "inpaint"
)

// JobRequest is what the generate screen submits.
type JobRequest struct {
	Provider       string `json:"provider"`
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negativePrompt"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Count          int    `json:"count"`
	Seed           int64  `json:"seed"`
	AssetType      string `json:"assetType"`
	Name           string `json:"name"`
	// Mode is "generate" (default) or "inpaint" for masked regeneration.
	Mode string `json:"mode,omitempty"`
	// TargetAssetID is the asset an inpaint job refines.
	TargetAssetID string `json:"targetAssetId,omitempty"`
	// BaseImage and Mask are PNG data URLs used by inpaint jobs.
	BaseImage string `json:"baseImage,omitempty"`
	Mask      string `json:"mask,omitempty"`
	// ImageName and MaskName are the uploaded ComfyUI filenames for a run.
	ImageName string `json:"-"`
	MaskName  string `json:"-"`
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// normalize snaps sizes to multiples of 8 (latent models) and clamps counts.
func (r JobRequest) normalize(defaultProvider string) (JobRequest, error) {
	out := r
	out.Prompt = strings.TrimSpace(r.Prompt)
	out.NegativePrompt = strings.TrimSpace(r.NegativePrompt)
	out.Name = strings.TrimSpace(r.Name)
	out.Provider = strings.ToLower(strings.TrimSpace(r.Provider))
	if out.Provider == "" {
		out.Provider = defaultProvider
	}
	if out.Provider == "" {
		out.Provider = ProviderComfyUI
	}
	if out.Prompt == "" {
		return out, apperr.New("INVALID_PROMPT", "请先填写提示词")
	}
	if out.Width <= 0 {
		out.Width = 512
	}
	if out.Height <= 0 {
		out.Height = 512
	}
	out.Width = (clampInt(out.Width, 64, 2048) / 8) * 8
	out.Height = (clampInt(out.Height, 64, 2048) / 8) * 8
	out.Count = clampInt(out.Count, 1, 4)
	out.Mode = strings.ToLower(strings.TrimSpace(r.Mode))
	if out.Mode == "" {
		out.Mode = JobModeGenerate
	}
	if out.Mode != JobModeGenerate && out.Mode != JobModeInpaint {
		return out, apperr.New("INVALID_MODE", "未知的生成模式").WithDetail(out.Mode)
	}
	if out.Mode == JobModeInpaint {
		if strings.TrimSpace(out.BaseImage) == "" || strings.TrimSpace(out.Mask) == "" {
			return out, apperr.New("INVALID_INPAINT", "局部重绘需要原图与蒙版")
		}
		if len(out.BaseImage) > 16<<20 || len(out.Mask) > 8<<20 {
			return out, apperr.New("INVALID_INPAINT", "局部重绘的原图或蒙版过大")
		}
		out.Count = 1
	} else {
		out.BaseImage = ""
		out.Mask = ""
		out.TargetAssetID = ""
	}
	out.TargetAssetID = strings.TrimSpace(out.TargetAssetID)
	return out, nil
}

// JobImage is one stored result image.
type JobImage struct {
	Index  int    `json:"index"`
	File   string `json:"file"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	// AdoptedAssetID records the asset this result was stored as, so the UI can
	// show it as adopted instead of offering duplicate copies.
	AdoptedAssetID string `json:"adoptedAssetId,omitempty"`
}

// Job is the persisted state of one generation request.
type Job struct {
	ID             string     `json:"id"`
	Provider       string     `json:"provider"`
	State          JobState   `json:"state"`
	Progress       int        `json:"progress"`
	Stage          string     `json:"stage"`
	Prompt         string     `json:"prompt"`
	NegativePrompt string     `json:"negativePrompt"`
	Width          int        `json:"width"`
	Height         int        `json:"height"`
	Count          int        `json:"count"`
	Seed           int64      `json:"seed"`
	AssetType      string     `json:"assetType"`
	Name           string     `json:"name"`
	Mode           string     `json:"mode,omitempty"`
	TargetAssetID  string     `json:"targetAssetId,omitempty"`
	BaseImage      string     `json:"baseImage,omitempty"`
	Mask           string     `json:"mask,omitempty"`
	Error          string     `json:"error,omitempty"`
	Images         []JobImage `json:"images"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

func (j *Job) clone() Job {
	out := *j
	out.Images = append([]JobImage(nil), j.Images...)
	return out
}
