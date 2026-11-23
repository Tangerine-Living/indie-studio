package generate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "golang.org/x/image/webp"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
	"pixelbench/internal/project"
)

// Manager owns the generation queue. Jobs run one at a time so a local
// ComfyUI instance is never hit with parallel paid calls.
type Manager struct {
	mu sync.Mutex
	// persistMu serialises job-file writes: an older snapshot must never land
	// after a newer one (a late "done" write could erase an adoption).
	persistMu  sync.Mutex
	jobs       map[string]*Job
	order      []string
	cancels    map[string]context.CancelFunc
	deleted    map[string]bool
	sem        chan struct{}
	settings   func() Settings
	providerFn func(Settings, string) (Provider, error)
	emit       func([]Job)
	link       func(jobID, file string) string
	root       string
	lastEmit   time.Time
	closed     bool
	wg         sync.WaitGroup
}

// NewManager creates a queue reading settings from settingsFn. link maps a
// stored image to a URL the webview can load; emit pushes jobs to the UI.
func NewManager(settings func() Settings, link func(jobID, file string) string, emit func([]Job)) *Manager {
	return &Manager{
		jobs:       map[string]*Job{},
		cancels:    map[string]context.CancelFunc{},
		deleted:    map[string]bool{},
		sem:        make(chan struct{}, 1),
		settings:   settings,
		providerFn: NewProvider,
		emit:       emit,
		link:       link,
	}
}

func jobsDir(root string) string {
	return filepath.Join(project.WorkbenchDir(root), "jobs")
}

func generatedDir(root, id string) string {
	return filepath.Join(project.WorkbenchDir(root), "generated", id)
}

// Restore loads persisted jobs of one project and marks interrupted ones.
func (m *Manager) Restore(root string) []Job {
	m.mu.Lock()
	m.root = root
	m.jobs = map[string]*Job{}
	m.order = nil
	m.deleted = map[string]bool{}
	m.mu.Unlock()

	entries, err := os.ReadDir(jobsDir(root))
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			var job Job
			if err := fsutil.ReadJSON(filepath.Join(jobsDir(root), entry.Name()), &job); err != nil {
				continue
			}
			if job.State == JobQueued || job.State == JobRunning {
				job.State = JobFailed
				job.Progress = 0
				job.Error = "应用重启，任务已中断；请重新提交"
				job.UpdatedAt = time.Now().UTC()
				_ = fsutil.WriteJSONAtomic(filepath.Join(jobsDir(root), job.ID+".json"), job, 0o644)
			}
			m.mu.Lock()
			m.jobs[job.ID] = &job
			m.order = append(m.order, job.ID)
			m.mu.Unlock()
		}
	}
	m.sortOrder()
	m.notify()
	return m.List()
}

func (m *Manager) sortOrder() {
	m.mu.Lock()
	defer m.mu.Unlock()
	sort.SliceStable(m.order, func(i, j int) bool {
		a := m.jobs[m.order[i]]
		b := m.jobs[m.order[j]]
		if a == nil || b == nil {
			return false
		}
		return a.CreatedAt.After(b.CreatedAt)
	})
}

// List returns newest-first copies with URLs resolved.
func (m *Manager) List() []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshotLocked()
}

func (m *Manager) snapshotLocked() []Job {
	out := make([]Job, 0, len(m.order))
	for _, id := range m.order {
		job := m.jobs[id]
		if job == nil {
			continue
		}
		copied := job.clone()
		if m.link != nil {
			for i := range copied.Images {
				copied.Images[i].URL = m.link(copied.ID, copied.Images[i].File)
			}
		}
		out = append(out, copied)
	}
	return out
}

func (m *Manager) notify() {
	m.mu.Lock()
	if m.closed || m.emit == nil {
		m.mu.Unlock()
		return
	}
	m.lastEmit = time.Now()
	jobs := m.snapshotLocked()
	emit := m.emit
	m.mu.Unlock()
	emit(jobs)
}

// Submit validates a request, persists it and starts the worker.
func (m *Manager) Submit(root string, req JobRequest) (Job, error) {
	settings := m.settings()
	normalized, err := req.normalize(settings.DefaultProvider)
	if err != nil {
		return Job{}, err
	}
	now := time.Now().UTC()
	job := &Job{
		ID:             project.NewID(),
		Provider:       normalized.Provider,
		State:          JobQueued,
		Progress:       0,
		Stage:          "排队中",
		Prompt:         normalized.Prompt,
		NegativePrompt: normalized.NegativePrompt,
		Width:          normalized.Width,
		Height:         normalized.Height,
		Count:          normalized.Count,
		Seed:           normalized.Seed,
		AssetType:      normalized.AssetType,
		Name:           normalized.Name,
		Mode:           normalized.Mode,
		TargetAssetID:  normalized.TargetAssetID,
		BaseImage:      normalized.BaseImage,
		Mask:           normalized.Mask,
		Images:         []JobImage{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	m.mu.Lock()
	if m.root != "" && !sameRoot(m.root, root) {
		m.mu.Unlock()
		return Job{}, apperr.New("PROJECT_CHANGED", "项目已切换，请重新提交")
	}
	if m.root == "" {
		m.root = root
	}
	m.jobs[job.ID] = job
	m.order = append(m.order, job.ID)
	m.mu.Unlock()

	if err := m.persist(job); err != nil {
		m.mu.Lock()
		delete(m.jobs, job.ID)
		m.removeOrderLocked(job.ID)
		m.mu.Unlock()
		return Job{}, err
	}
	m.notify()
	m.wg.Add(1)
	go m.run(root, job.ID)
	return job.clone(), nil
}

func (m *Manager) removeOrderLocked(id string) {
	filtered := m.order[:0]
	for _, existing := range m.order {
		if existing != id {
			filtered = append(filtered, existing)
		}
	}
	m.order = filtered
}

func sameRoot(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// Cancel stops a queued or running job.
func (m *Manager) Cancel(id string) (Job, error) {
	m.mu.Lock()
	job := m.jobs[id]
	if job == nil {
		m.mu.Unlock()
		return Job{}, apperr.New("JOB_NOT_FOUND", "任务不存在").WithDetail(id)
	}
	cancel := m.cancels[id]
	state := job.State
	m.mu.Unlock()
	if state != JobQueued && state != JobRunning {
		return job.clone(), nil
	}
	if cancel != nil {
		cancel()
	}
	return m.update(id, func(job *Job) {
		if job.State == JobQueued || job.State == JobRunning {
			job.State = JobCanceled
			job.Stage = "已取消"
			job.Progress = 0
			job.BaseImage = ""
			job.Mask = ""
		}
	})
}

// Delete removes a finished, failed or canceled job together with its persisted
// JSON and generated images. Active jobs must be canceled first: deleting a
// running job could race the provider writing new files into the folder.
func (m *Manager) Delete(id string) error {
	if !project.ValidID(id) {
		return apperr.New("INVALID_PATH", "任务路径不合法")
	}
	m.mu.Lock()
	job := m.jobs[id]
	if job == nil {
		m.mu.Unlock()
		return apperr.New("JOB_NOT_FOUND", "任务不存在").WithDetail(id)
	}
	if job.State == JobQueued || job.State == JobRunning {
		m.mu.Unlock()
		return apperr.New("JOB_ACTIVE", "任务进行中，请先取消再删除")
	}
	root := m.root
	// Tombstone first so a cancel that is still unwinding cannot rewrite the
	// job file after the folder has been removed.
	m.deleted[id] = true
	delete(m.jobs, id)
	m.removeOrderLocked(id)
	delete(m.cancels, id)
	m.mu.Unlock()

	if root != "" {
		file := filepath.Join(jobsDir(root), id+".json")
		if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
			return apperr.Wrap("IO", "无法删除任务文件", err)
		}
		if err := os.RemoveAll(generatedDir(root, id)); err != nil {
			return apperr.Wrap("IO", "无法删除生成结果", err)
		}
	}
	m.notify()
	return nil
}

// Close cancels everything; jobs interrupted this way stay resumable by hand.
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	cancels := make([]context.CancelFunc, 0, len(m.cancels))
	for _, cancel := range m.cancels {
		cancels = append(cancels, cancel)
	}
	m.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	m.wg.Wait()
}

// Detach stops all jobs and forgets the project so another one can open.
func (m *Manager) Detach() {
	m.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(m.cancels))
	for _, cancel := range m.cancels {
		cancels = append(cancels, cancel)
	}
	m.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	m.wg.Wait()
	m.mu.Lock()
	m.jobs = map[string]*Job{}
	m.order = nil
	m.cancels = map[string]context.CancelFunc{}
	m.deleted = map[string]bool{}
	m.root = ""
	m.lastEmit = time.Time{}
	m.mu.Unlock()
	m.notify()
}

func (m *Manager) run(root, id string) {
	defer m.wg.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	m.mu.Lock()
	if m.jobs[id] == nil || m.jobs[id].State != JobQueued {
		m.mu.Unlock()
		return
	}
	m.cancels[id] = cancel
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.cancels, id)
		m.mu.Unlock()
	}()

	select {
	case m.sem <- struct{}{}:
		defer func() { <-m.sem }()
	case <-ctx.Done():
		m.fail(id, ctx.Err())
		return
	}

	m.mu.Lock()
	job := m.jobs[id]
	if job == nil || job.State == JobCanceled {
		m.mu.Unlock()
		return
	}
	job.State = JobRunning
	job.Stage = "连接生成服务"
	job.Progress = 3
	providerID := job.Provider
	req := JobRequest{
		Provider:       job.Provider,
		Prompt:         job.Prompt,
		NegativePrompt: job.NegativePrompt,
		Width:          job.Width,
		Height:         job.Height,
		Count:          job.Count,
		Seed:           job.Seed,
		AssetType:      job.AssetType,
		Name:           job.Name,
		Mode:           job.Mode,
		TargetAssetID:  job.TargetAssetID,
		BaseImage:      job.BaseImage,
		Mask:           job.Mask,
	}
	m.mu.Unlock()
	m.persistCurrent(id)
	m.notify()

	settings := m.settings()
	provider, err := m.providerFn(settings, providerID)
	if err != nil {
		m.fail(id, err)
		return
	}
	images, err := provider.Generate(ctx, req, func(percent int, stage string) {
		m.progress(id, percent, stage)
	})
	if err != nil {
		m.fail(id, err)
		return
	}

	m.mu.Lock()
	gone := m.deleted[id] || m.jobs[id] == nil
	m.mu.Unlock()
	if gone {
		return
	}
	saved, saveErr := m.storeImages(root, id, images, req.Count)
	if saveErr != nil {
		m.fail(id, saveErr)
		return
	}
	m.update(id, func(job *Job) {
		job.Images = saved
		job.State = JobDone
		job.Progress = 100
		job.Stage = "完成"
		// The raw base/mask are only needed while running; keep job files small.
		job.BaseImage = ""
		job.Mask = ""
	})
}

func (m *Manager) fail(id string, err error) {
	if err == nil {
		err = apperr.New("PROVIDER_ERROR", "生成失败")
	}
	m.update(id, func(job *Job) {
		if job.State == JobCanceled {
			return
		}
		switch {
		case errors.Is(err, context.Canceled):
			job.State = JobCanceled
			job.Stage = "已取消"
		case errors.Is(err, context.DeadlineExceeded):
			job.State = JobFailed
			job.Stage = "超时"
			job.Error = "生成超时，请重试或检查生成服务"
		default:
			job.State = JobFailed
			job.Stage = "失败"
			var appErr *apperr.Error
			if errors.As(err, &appErr) {
				job.Error = appErr.Message
				if appErr.Detail != "" {
					job.Error += "\n" + appErr.Detail
				}
			} else {
				job.Error = err.Error()
			}
		}
		job.Progress = 0
		job.BaseImage = ""
		job.Mask = ""
	})
}

// MarkAdopted records which asset now holds one generated result. The job file
// keeps the link across restarts so the same image is not adopted twice.
func (m *Manager) MarkAdopted(id string, index int, assetID string) (Job, error) {
	if assetID == "" {
		return Job{}, apperr.New("INVALID_ASSET", "素材 ID 为空")
	}
	return m.update(id, func(job *Job) {
		if index < 0 || index >= len(job.Images) {
			return
		}
		job.Images[index].AdoptedAssetID = assetID
	})
}

func (m *Manager) progress(id string, percent int, stage string) {
	m.update(id, func(job *Job) {
		if job.State != JobRunning {
			return
		}
		if percent < 0 {
			percent = 0
		}
		if percent > 99 {
			percent = 99
		}
		job.Progress = percent
		if stage != "" {
			job.Stage = stage
		}
	})
}

func (m *Manager) update(id string, mutate func(*Job)) (Job, error) {
	m.mu.Lock()
	job := m.jobs[id]
	if job == nil {
		m.mu.Unlock()
		return Job{}, apperr.New("JOB_NOT_FOUND", "任务不存在").WithDetail(id)
	}
	mutate(job)
	job.UpdatedAt = time.Now().UTC()
	running := job.State == JobRunning
	m.mu.Unlock()
	// Persist on state transitions; progress is throttled.
	if !running {
		_ = m.persistCurrent(id)
		m.notify()
	} else {
		m.mu.Lock()
		due := time.Since(m.lastEmit) > 400*time.Millisecond
		m.mu.Unlock()
		if due {
			m.notify()
		}
	}
	return m.copyJob(id), nil
}

func (m *Manager) copyJob(id string) Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[id]
	if job == nil {
		return Job{}
	}
	return job.clone()
}

func (m *Manager) persistCurrent(id string) error {
	m.mu.Lock()
	job := m.jobs[id]
	if job == nil || m.root == "" || m.deleted[id] {
		m.mu.Unlock()
		return nil
	}
	snapshot := job.clone()
	root := m.root
	m.mu.Unlock()
	return m.persist(&snapshot, root)
}

func (m *Manager) persist(job *Job, root ...string) error {
	base := m.root
	if len(root) > 0 {
		base = root[0]
	}
	if base == "" {
		return nil
	}
	m.persistMu.Lock()
	defer m.persistMu.Unlock()
	if err := os.MkdirAll(jobsDir(base), 0o755); err != nil {
		return apperr.Wrap("IO", "无法创建任务目录", err)
	}
	return fsutil.WriteJSONAtomic(filepath.Join(jobsDir(base), job.ID+".json"), job, 0o644)
}

func (m *Manager) storeImages(root, id string, images []GeneratedImage, count int) ([]JobImage, error) {
	dir := generatedDir(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, apperr.Wrap("IO", "无法创建生成结果目录", err)
	}
	saved := make([]JobImage, 0, len(images))
	for index, item := range images {
		if len(saved) >= count {
			break
		}
		src, _, err := image.Decode(bytes.NewReader(item.Data))
		if err != nil {
			continue
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, src); err != nil {
			continue
		}
		name := fmt.Sprintf("%03d.png", index)
		if err := fsutil.WriteFileAtomic(filepath.Join(dir, name), buf.Bytes(), 0o644); err != nil {
			return nil, apperr.Wrap("IO", "无法写入生成结果", err)
		}
		bounds := src.Bounds()
		saved = append(saved, JobImage{
			Index:  len(saved),
			File:   name,
			Width:  bounds.Dx(),
			Height: bounds.Dy(),
		})
	}
	if len(saved) == 0 {
		return nil, apperr.New("PROVIDER_ERROR", "生成结果不是可识别的图片")
	}
	return saved, nil
}

// GeneratedFile resolves one stored job image to an absolute path, rejecting
// anything outside the job directory.
func GeneratedFile(root, jobID, name string) (string, error) {
	if !project.ValidID(jobID) {
		return "", apperr.New("INVALID_PATH", "任务路径不合法")
	}
	clean := filepath.Base(filepath.FromSlash(name))
	if clean != name || !strings.HasSuffix(strings.ToLower(clean), ".png") {
		return "", apperr.New("INVALID_PATH", "结果文件名不合法")
	}
	dir := generatedDir(root, jobID)
	abs := filepath.Join(dir, clean)
	if !fsutil.Within(dir, abs) {
		return "", apperr.New("INVALID_PATH", "结果文件路径不合法")
	}
	if _, err := os.Stat(abs); err != nil {
		return "", apperr.New("JOB_IMAGE_MISSING", "生成结果文件不存在")
	}
	return abs, nil
}
