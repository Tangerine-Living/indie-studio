package generate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pixelbench/internal/fsutil"
)

type fakeProvider struct {
	wait   bool
	err    error
	images [][]byte
	onGen  func(ProgressFunc)
}

func (f *fakeProvider) ID() string { return ProviderComfyUI }

func (f *fakeProvider) Test(context.Context) (TestResult, error) {
	return TestResult{OK: true, Message: "ok"}, nil
}

func (f *fakeProvider) Generate(ctx context.Context, req JobRequest, progress ProgressFunc) ([]GeneratedImage, error) {
	if f.onGen != nil {
		f.onGen(progress)
	}
	if f.wait {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if f.err != nil {
		return nil, f.err
	}
	if progress != nil {
		progress(80, "下载")
	}
	out := make([]GeneratedImage, 0, len(f.images))
	for _, data := range f.images {
		out = append(out, GeneratedImage{Data: data, Format: "png"})
	}
	return out, nil
}

func testManager(t *testing.T, provider Provider) *Manager {
	t.Helper()
	manager := NewManager(
		func() Settings { return defaults() },
		func(jobID, file string) string { return "/pb-gen/" + jobID + "/" + file },
		nil,
	)
	manager.providerFn = func(Settings, string) (Provider, error) { return provider, nil }
	t.Cleanup(manager.Close)
	return manager
}

func waitFor(t *testing.T, manager *Manager, id string, state JobState) Job {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, job := range manager.List() {
			if job.ID == id && job.State == state {
				return job
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s did not reach %s", id, state)
	return Job{}
}

func TestManagerRunsJobAndStoresImages(t *testing.T) {
	root := t.TempDir()
	provider := &fakeProvider{images: [][]byte{tinyPNG(t)}}
	manager := testManager(t, provider)

	job, err := manager.Submit(root, JobRequest{Prompt: "杂货铺", Width: 512, Height: 512, Count: 1, Seed: 9})
	if err != nil {
		t.Fatal(err)
	}
	done := waitFor(t, manager, job.ID, JobDone)
	if done.Progress != 100 || len(done.Images) != 1 {
		t.Fatalf("done job = %+v", done)
	}
	image := done.Images[0]
	if image.Width != 8 || image.Height != 6 {
		t.Fatalf("image size = %d×%d", image.Width, image.Height)
	}
	if image.URL != "/pb-gen/"+job.ID+"/000.png" {
		t.Fatalf("url = %q", image.URL)
	}
	abs, err := GeneratedFile(root, job.ID, image.File)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("image file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(jobsDir(root), job.ID+".json")); err != nil {
		t.Fatalf("job file: %v", err)
	}
}

func TestMarkAdoptedLinksImageToAssetAndPersists(t *testing.T) {
	root := t.TempDir()
	manager := testManager(t, &fakeProvider{images: [][]byte{tinyPNG(t)}})
	job, err := manager.Submit(root, JobRequest{Prompt: "精灵", Width: 512, Height: 512, Count: 1})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, manager, job.ID, JobDone)

	updated, err := manager.MarkAdopted(job.ID, 0, "01JASSET")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Images[0].AdoptedAssetID != "01JASSET" {
		t.Fatalf("adopted = %q", updated.Images[0].AdoptedAssetID)
	}

	var stored Job
	if err := fsutil.ReadJSON(filepath.Join(jobsDir(root), job.ID+".json"), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Images[0].AdoptedAssetID != "01JASSET" {
		t.Fatalf("persisted adopted = %q", stored.Images[0].AdoptedAssetID)
	}
}

func TestManagerCancelStopsRunningJob(t *testing.T) {
	root := t.TempDir()
	manager := testManager(t, &fakeProvider{wait: true})
	job, err := manager.Submit(root, JobRequest{Prompt: "慢任务", Width: 256, Height: 256, Count: 1})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, manager, job.ID, JobRunning)
	canceled, err := manager.Cancel(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if canceled.State != JobCanceled && canceled.State != JobRunning {
		t.Fatalf("state = %s", canceled.State)
	}
	waitFor(t, manager, job.ID, JobCanceled)
}

func TestManagerDeleteRemovesTerminalJob(t *testing.T) {
	root := t.TempDir()
	manager := testManager(t, &fakeProvider{images: [][]byte{tinyPNG(t)}})
	job, err := manager.Submit(root, JobRequest{Prompt: "杂货铺", Width: 512, Height: 512, Count: 1})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, manager, job.ID, JobDone)

	if err := manager.Delete(job.ID); err != nil {
		t.Fatal(err)
	}
	if len(manager.List()) != 0 {
		t.Fatalf("jobs after delete = %d", len(manager.List()))
	}
	if _, err := os.Stat(filepath.Join(jobsDir(root), job.ID+".json")); !os.IsNotExist(err) {
		t.Fatalf("job json still exists: %v", err)
	}
	if _, err := os.Stat(generatedDir(root, job.ID)); !os.IsNotExist(err) {
		t.Fatalf("generated dir still exists: %v", err)
	}
	if err := manager.Delete(job.ID); err == nil {
		t.Fatal("expected second delete to fail")
	}
}

func TestManagerDeleteRejectsActiveJob(t *testing.T) {
	root := t.TempDir()
	manager := testManager(t, &fakeProvider{wait: true})
	job, err := manager.Submit(root, JobRequest{Prompt: "慢任务", Width: 256, Height: 256, Count: 1})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, manager, job.ID, JobRunning)
	if err := manager.Delete(job.ID); err == nil {
		t.Fatal("expected active job deletion to be rejected")
	}
	if _, err := manager.Cancel(job.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, manager, job.ID, JobCanceled)
	if err := manager.Delete(job.ID); err != nil {
		t.Fatal(err)
	}
}

func TestManagerRejectsEmptyPrompt(t *testing.T) {
	manager := testManager(t, &fakeProvider{})
	if _, err := manager.Submit(t.TempDir(), JobRequest{Prompt: "   "}); err == nil {
		t.Fatal("expected prompt validation error")
	}
}

func TestRestoreMarksInterruptedJobs(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(jobsDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	interrupted := Job{
		ID:        "job1",
		Provider:  ProviderComfyUI,
		State:     JobRunning,
		Prompt:    "x",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := os.WriteFile(
		filepath.Join(jobsDir(root), "job1.json"),
		mustJSON(t, interrupted),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	manager := testManager(t, &fakeProvider{})
	jobs := manager.Restore(root)
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d", len(jobs))
	}
	if jobs[0].State != JobFailed || jobs[0].Error == "" {
		t.Fatalf("restored job = %+v", jobs[0])
	}
}

func TestGeneratedFileRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if _, err := GeneratedFile(root, "job1", "../x.png"); err == nil {
		t.Fatal("expected traversal rejection")
	}
	if _, err := GeneratedFile(root, "../job", "x.png"); err == nil {
		t.Fatal("expected job id rejection")
	}
	if _, err := GeneratedFile(root, "job1", "x.txt"); err == nil {
		t.Fatal("expected extension rejection")
	}
}
