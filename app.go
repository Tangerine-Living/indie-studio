package main

import (
	"bytes"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image/png"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"pixelbench/internal/apperr"
	assetlib "pixelbench/internal/assets"
	"pixelbench/internal/editorprefs"
	mapexport "pixelbench/internal/export"
	"pixelbench/internal/fsutil"
	"pixelbench/internal/generate"
	"pixelbench/internal/generators"
	"pixelbench/internal/imageproc"
	"pixelbench/internal/logging"
	"pixelbench/internal/platform"
	"pixelbench/internal/project"
	"pixelbench/internal/update"
)

// Version is overridden at build time with -ldflags.
var Version = "0.1.0"

// AppInfo is shown in the UI and settings page.
type AppInfo struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Platform string `json:"platform"`
	LogPath  string `json:"logPath"`
	Mode     string `json:"mode"`
}

// Bootstrap is the single startup payload the frontend needs.
type Bootstrap struct {
	Info    AppInfo                 `json:"info"`
	Recent  []project.RecentProject `json:"recent"`
	Session *project.Session        `json:"session"`
}

// App is the Wails-bound application service.
type App struct {
	ctx        context.Context
	mu         sync.RWMutex
	store      *project.Store
	assetToken string
	gen        *generate.Manager
}

// NewApp creates the application service.
func NewApp() *App {
	app := &App{assetToken: randomToken()}
	app.gen = generate.NewManager(
		generate.LoadSettings,
		app.generatedURL,
		app.emitJobs,
	)
	return app
}

// emitJobs pushes the queue to the generate screen.
func (a *App) emitJobs(jobs []generate.Job) {
	if a.ctx == nil {
		return
	}
	if jobs == nil {
		jobs = []generate.Job{}
	}
	wailsruntime.EventsEmit(a.ctx, "ai:jobs", jobs)
}

// generatedURL builds the token-protected URL of one job image.
func (a *App) generatedURL(jobID, file string) string {
	store := a.currentStore()
	if store == nil {
		return ""
	}
	abs, err := generate.GeneratedFile(store.Root(), jobID, file)
	if err != nil {
		return ""
	}
	info, statErr := os.Stat(abs)
	if statErr != nil {
		return ""
	}
	return fmt.Sprintf("/pb-gen/%s/%s/%s?v=%d",
		a.assetToken, url.PathEscape(jobID), url.PathEscape(file), info.ModTime().Unix())
}

func randomToken() string {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf)
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if _, err := logging.Init(); err != nil {
		slog.Error("cannot initialise logging", "error", err)
	}
	slog.Info("Indie Studio started", "version", Version, "platform", runtime.GOOS)
}

func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	store := a.store
	a.store = nil
	a.mu.Unlock()
	if a.gen != nil {
		a.gen.Close()
	}
	if store != nil {
		_ = store.Close()
	}
	logging.Close()
}

func (a *App) currentStore() *project.Store {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.store
}

func (a *App) replaceStore(next *project.Store) {
	a.mu.Lock()
	previous := a.store
	a.store = next
	a.mu.Unlock()
	if previous != nil {
		_ = previous.Close()
	}
}

// GetBootstrap returns app info, recent projects and the active session.
func (a *App) GetBootstrap() (out Bootstrap, err error) {
	defer logging.Catch("GetBootstrap", &err)
	out.Info = AppInfo{
		Name:     "Indie Studio",
		Version:  Version,
		Platform: runtime.GOOS,
		LogPath:  logging.Dir(),
		Mode:     "desktop",
	}
	out.Recent = project.LoadRecent()
	if out.Recent == nil {
		out.Recent = []project.RecentProject{}
	}
	if store := a.currentStore(); store != nil {
		session := store.Session()
		out.Session = &session
	}
	return out, nil
}

// GetVersion returns the application version.
func (a *App) GetVersion() string { return Version }

// CreateProject creates and opens a project.
func (a *App) CreateProject(opts project.CreateOptions) (session project.Session, err error) {
	defer logging.Catch("CreateProject", &err)
	store, err := project.Create(opts)
	if err != nil {
		return project.Session{}, err
	}
	a.replaceStore(store)
	a.gen.Restore(store.Root())
	return store.Session(), nil
}

// OpenProject opens an existing project folder (or project.json).
func (a *App) OpenProject(projectPath string, force bool) (session project.Session, err error) {
	defer logging.Catch("OpenProject", &err)
	store, err := project.Open(projectPath, force)
	if err != nil {
		return project.Session{}, err
	}
	a.replaceStore(store)
	a.gen.Restore(store.Root())
	return store.Session(), nil
}

// CloseProject releases the active project.
func (a *App) CloseProject() (err error) {
	defer logging.Catch("CloseProject", &err)
	a.mu.Lock()
	store := a.store
	a.store = nil
	a.mu.Unlock()
	if a.gen != nil {
		a.gen.Detach()
	}
	if store != nil {
		return store.Close()
	}
	return nil
}

// GetSession returns the active session, or null.
func (a *App) GetSession() (session *project.Session, err error) {
	defer logging.Catch("GetSession", &err)
	store := a.currentStore()
	if store == nil {
		return nil, nil
	}
	current := store.Session()
	return &current, nil
}

// SaveProject writes the project manifest.
func (a *App) SaveProject(p project.Project) (session project.Session, err error) {
	defer logging.Catch("SaveProject", &err)
	store := a.currentStore()
	if store == nil {
		return project.Session{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return store.SaveProject(p)
}

// ListQuests returns all quests in the active project.
func (a *App) ListQuests() (quests []project.Quest, err error) {
	defer logging.Catch("ListQuests", &err)
	store := a.currentStore()
	if store == nil {
		return []project.Quest{}, nil
	}
	return store.ListQuests()
}

// SaveQuest writes one quest.
func (a *App) SaveQuest(quest project.Quest) (saved project.Quest, err error) {
	defer logging.Catch("SaveQuest", &err)
	store := a.currentStore()
	if store == nil {
		return project.Quest{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return store.SaveQuest(quest)
}

// DeleteQuest moves a quest to the project trash.
func (a *App) DeleteQuest(id string) (err error) {
	defer logging.Catch("DeleteQuest", &err)
	store := a.currentStore()
	if store == nil {
		return apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return store.DeleteQuest(id)
}

// ListEntities returns all setting entries in the active project.
func (a *App) ListEntities() (entities []project.Entity, err error) {
	defer logging.Catch("ListEntities", &err)
	store := a.currentStore()
	if store == nil {
		return []project.Entity{}, nil
	}
	return store.ListEntities()
}

// SaveEntity writes one setting entry.
func (a *App) SaveEntity(entity project.Entity) (saved project.Entity, err error) {
	defer logging.Catch("SaveEntity", &err)
	store := a.currentStore()
	if store == nil {
		return project.Entity{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return store.SaveEntity(entity)
}

// DeleteEntity moves one setting entry to the project trash.
func (a *App) DeleteEntity(id string) (err error) {
	defer logging.Catch("DeleteEntity", &err)
	store := a.currentStore()
	if store == nil {
		return apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return store.DeleteEntity(id)
}

// ListTables returns every game-data table in the active project.
func (a *App) ListTables() (tables []project.DataTable, err error) {
	defer logging.Catch("ListTables", &err)
	store := a.currentStore()
	if store == nil {
		return []project.DataTable{}, nil
	}
	return store.ListTables()
}

// SaveTable writes one game-data table.
func (a *App) SaveTable(table project.DataTable) (saved project.DataTable, err error) {
	defer logging.Catch("SaveTable", &err)
	store := a.currentStore()
	if store == nil {
		return project.DataTable{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return store.SaveTable(table)
}

// DeleteTable moves one game-data table to the project trash.
func (a *App) DeleteTable(id string) (err error) {
	defer logging.Catch("DeleteTable", &err)
	store := a.currentStore()
	if store == nil {
		return apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return store.DeleteTable(id)
}

// ExportTable writes JSON and CSV under exports/tables/<key>/.
func (a *App) ExportTable(id string) (result mapexport.TableExportResult, err error) {
	defer logging.Catch("ExportTable", &err)
	store := a.currentStore()
	if store == nil {
		return mapexport.TableExportResult{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return mapexport.ExportTable(store, id)
}

// ListAssets returns all assets in the active project.
func (a *App) ListAssets() (assets []project.Asset, err error) {
	defer logging.Catch("ListAssets", &err)
	store := a.currentStore()
	if store == nil {
		return []project.Asset{}, nil
	}
	return store.ListAssets()
}

// SaveAsset writes one asset manifest.
func (a *App) SaveAsset(asset project.Asset) (saved project.Asset, err error) {
	defer logging.Catch("SaveAsset", &err)
	store := a.currentStore()
	if store == nil {
		return project.Asset{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return store.SaveAsset(asset)
}

// DeleteAsset moves an asset directory into the project trash.
func (a *App) DeleteAsset(id string) (err error) {
	defer logging.Catch("DeleteAsset", &err)
	store := a.currentStore()
	if store == nil {
		return apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return store.DeleteAsset(id)
}

// ImportOptions selects the files and the default type for a batch import.
type ImportOptions struct {
	Paths []string `json:"paths"`
	Type  string   `json:"type"`
}

// ImportAssets copies image files into the project, deduplicating by hash.
func (a *App) ImportAssets(opts ImportOptions) (summary assetlib.Summary, err error) {
	defer logging.Catch("ImportAssets", &err)
	store := a.currentStore()
	if store == nil {
		return assetlib.Summary{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return assetlib.Import(store, opts.Paths, opts.Type)
}

// CreateBlankAsset creates an empty draft asset the editor can draw on.
func (a *App) CreateBlankAsset(input assetlib.BlankAssetInput) (asset project.Asset, err error) {
	defer logging.Catch("CreateBlankAsset", &err)
	store := a.currentStore()
	if store == nil {
		return project.Asset{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return assetlib.CreateBlank(store, input)
}

// ListPalettes returns all palettes in the active project.
func (a *App) ListPalettes() (palettes []project.Palette, err error) {
	defer logging.Catch("ListPalettes", &err)
	store := a.currentStore()
	if store == nil {
		return []project.Palette{}, nil
	}
	return store.ListPalettes()
}

// SavePalette writes one palette.
func (a *App) SavePalette(palette project.Palette) (saved project.Palette, err error) {
	defer logging.Catch("SavePalette", &err)
	store := a.currentStore()
	if store == nil {
		return project.Palette{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return store.SavePalette(palette)
}

// DeletePalette moves an unused, non-default palette into the trash.
func (a *App) DeletePalette(id string) (err error) {
	defer logging.Catch("DeletePalette", &err)
	store := a.currentStore()
	if store == nil {
		return apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return store.DeletePalette(id)
}

// ListMaps returns map summaries for the maps screen.
func (a *App) ListMaps() (maps []project.MapSummary, err error) {
	defer logging.Catch("ListMaps", &err)
	store := a.currentStore()
	if store == nil {
		return []project.MapSummary{}, nil
	}
	return store.ListMaps()
}

// LoadMap returns one full map document.
func (a *App) LoadMap(id string) (m project.GameMap, err error) {
	defer logging.Catch("LoadMap", &err)
	store := a.currentStore()
	if store == nil {
		return project.GameMap{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return store.LoadMap(id)
}

// SaveMap writes one map document atomically.
func (a *App) SaveMap(m project.GameMap) (saved project.GameMap, err error) {
	defer logging.Catch("SaveMap", &err)
	store := a.currentStore()
	if store == nil {
		return project.GameMap{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return store.SaveMap(m)
}

// DeleteMap moves a map to the project trash.
func (a *App) DeleteMap(id string) (err error) {
	defer logging.Catch("DeleteMap", &err)
	store := a.currentStore()
	if store == nil {
		return apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return store.DeleteMap(id)
}

// ExportMap writes Tiled, Godot or both formats into exports/maps/<name>/.
func (a *App) ExportMap(id, format string) (result mapexport.MapExportResult, err error) {
	defer logging.Catch("ExportMap", &err)
	store := a.currentStore()
	if store == nil {
		return mapexport.MapExportResult{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return mapexport.ExportMap(store, id, format)
}

// PickDirectory opens the native folder picker.
func (a *App) PickDirectory(title string) (dir string, err error) {
	defer logging.Catch("PickDirectory", &err)
	if a.ctx == nil {
		return "", apperr.New("NOT_READY", "窗口尚未就绪")
	}
	if strings.TrimSpace(title) == "" {
		title = "选择文件夹"
	}
	opts := wailsruntime.OpenDialogOptions{Title: title}
	if home, err := os.UserHomeDir(); err == nil {
		opts.DefaultDirectory = home
	}
	dir, err = wailsruntime.OpenDirectoryDialog(a.ctx, opts)
	if err != nil {
		return "", apperr.Wrap("DIALOG", "无法打开文件夹选择器", err)
	}
	return dir, nil
}

// PickProjectFile opens the native project picker.
func (a *App) PickProjectFile() (file string, err error) {
	defer logging.Catch("PickProjectFile", &err)
	if a.ctx == nil {
		return "", apperr.New("NOT_READY", "窗口尚未就绪")
	}
	opts := wailsruntime.OpenDialogOptions{
		Title: "打开 Indie Studio 项目",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Indie Studio 项目 (project.json)", Pattern: "project.json"},
			{DisplayName: "JSON 文件", Pattern: "*.json"},
		},
	}
	if home, err := os.UserHomeDir(); err == nil {
		opts.DefaultDirectory = home
	}
	file, err = wailsruntime.OpenFileDialog(a.ctx, opts)
	if err != nil {
		return "", apperr.Wrap("DIALOG", "无法打开项目选择器", err)
	}
	return file, nil
}

// PickImageFiles opens the native image picker for asset import.
func (a *App) PickImageFiles() (files []string, err error) {
	defer logging.Catch("PickImageFiles", &err)
	if a.ctx == nil {
		return []string{}, apperr.New("NOT_READY", "窗口尚未就绪")
	}
	files, err = wailsruntime.OpenMultipleFilesDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "导入素材",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "图片 (PNG / JPEG / GIF / WebP / BMP)", Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.webp;*.bmp"},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	})
	if err != nil {
		return []string{}, apperr.Wrap("DIALOG", "无法打开图片选择器", err)
	}
	if files == nil {
		return []string{}, nil
	}
	return files, nil
}

// OpenProjectFolder reveals the project folder in Explorer.
func (a *App) OpenProjectFolder() (err error) {
	defer logging.Catch("OpenProjectFolder", &err)
	store := a.currentStore()
	if store == nil {
		return apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return platform.OpenPath(store.Root())
}

// OpenLogsFolder reveals the log directory.
func (a *App) OpenLogsFolder() (err error) {
	defer logging.Catch("OpenLogsFolder", &err)
	dir := logging.Dir()
	if dir == "" {
		if _, initErr := logging.Init(); initErr != nil {
			return apperr.Wrap("LOG", "无法初始化日志目录", initErr)
		}
		dir = logging.Dir()
	}
	if dir == "" {
		return apperr.New("LOG", "日志目录不可用")
	}
	return platform.OpenPath(dir)
}

// UpdatePublicKey can still be injected at build time with -ldflags. An empty
// key falls back to the embedded release key. A build with neither refuses
// every manifest instead of trusting unsigned updates.
var UpdatePublicKey = ""

//go:embed release/public.b64
var embeddedUpdatePublicKey string

// UpdateRepository is the public repository that hosts signed manifests and
// installers. It is part of the product, not a user-editable setting.
const UpdateRepository = update.Repository

// GetUpdateSettings returns the persisted updater configuration. The manifest
// URL is derived from the fixed repository in update.Check, never from input.
func (a *App) GetUpdateSettings() update.Settings {
	settings := update.LoadSettings()
	// The source is fixed by the product; only an explicit environment
	// override (used by tests and local release dry-runs) may change it.
	settings.ManifestURL = strings.TrimSpace(os.Getenv("PIXELBENCH_UPDATE_URL"))
	settings.SignatureURL = ""
	return settings
}

// SaveUpdateSettings persists the channel and auto-check flags. The manifest
// and signature URLs are fixed by the product and are not accepted from the UI.
func (a *App) SaveUpdateSettings(settings update.Settings) (saved update.Settings, err error) {
	defer logging.Catch("SaveUpdateSettings", &err)
	current := update.LoadSettings()
	current.Channel = settings.Channel
	current.AutoCheck = settings.AutoCheck
	// Drop any URL a previous version let the user store.
	current.ManifestURL = ""
	current.SignatureURL = ""
	return update.SaveSettings(current)
}

// updateKey resolves the signing key: ldflags/env first, then the embedded
// public key that ships with the source.
func updateKey() string {
	if key := strings.TrimSpace(UpdatePublicKey); key != "" {
		return key
	}
	if env := strings.TrimSpace(os.Getenv("PIXELBENCH_UPDATE_PUBKEY")); env != "" {
		return env
	}
	return strings.TrimSpace(embeddedUpdatePublicKey)
}

// CheckForUpdates fetches and verifies the manifest for the chosen channel.
func (a *App) CheckForUpdates() (status update.Status, err error) {
	defer logging.Catch("CheckForUpdates", &err)
	settings := a.GetUpdateSettings()
	client := update.New(Version, updateKey(), settings)
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return client.Check(ctx)
}

// DownloadUpdate downloads the installer, verifies its SHA-256 and returns the
// local path. Installing stays a deliberate user action.
func (a *App) DownloadUpdate(status update.Status) (path string, err error) {
	defer logging.Catch("DownloadUpdate", &err)
	settings := a.GetUpdateSettings()
	client := update.New(Version, updateKey(), settings)
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return client.Download(ctx, status)
}

// AssetSourceInfo is the editor-facing base image of one asset.
type AssetSourceInfo struct {
	File string `json:"file"`
	URL  string `json:"url"`
}

// GetAssetSource returns the image the editor opens: the first frame of an
// animated asset, else the current version, else the original source file,
// else the thumbnail. Returning one struct keeps the JS shape stable.
func (a *App) GetAssetSource(assetID string) (info AssetSourceInfo, err error) {
	defer logging.Catch("GetAssetSource", &err)
	store := a.currentStore()
	if store == nil {
		return AssetSourceInfo{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	asset, err := store.GetAsset(assetID)
	if err != nil {
		return AssetSourceInfo{}, err
	}
	build := func(rel string) (AssetSourceInfo, error) {
		sourceURL, urlErr := a.GetAssetURL(assetID, rel)
		if urlErr != nil {
			return AssetSourceInfo{}, urlErr
		}
		return AssetSourceInfo{File: rel, URL: sourceURL}, nil
	}
	// An animated asset reopens on its first derived frame, not the sheet.
	if asset.Animation != nil {
		framesPath := filepath.Join(project.AssetDir(store.Root(), assetID), "frames")
		entries, readErr := os.ReadDir(framesPath)
		if readErr == nil {
			names := make([]string, 0, len(entries))
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".png") {
					names = append(names, entry.Name())
				}
			}
			sort.Strings(names)
			if len(names) > 0 {
				return build(path.Join("frames", names[0]))
			}
		}
	}
	// Prefer the current version: it is what the asset detail shows, uses an
	// ASCII path, and works for layered/generated assets whose source/ may be
	// empty or hold separate layers.
	if asset.CurrentVersion != "" {
		rel := path.Join("versions", asset.CurrentVersion, "output.png")
		if _, statErr := os.Stat(filepath.Join(project.AssetDir(store.Root(), assetID), filepath.FromSlash(rel))); statErr == nil {
			return build(rel)
		}
	}
	entries, readErr := os.ReadDir(filepath.Join(project.AssetDir(store.Root(), assetID), "source"))
	if readErr == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			return build(path.Join("source", entry.Name()))
		}
	}
	return build("thumb.png")
}

func decodeDataURL(dataURL string) ([]byte, error) {
	data := strings.TrimSpace(dataURL)
	if strings.HasPrefix(data, "data:") {
		comma := strings.Index(data, ",")
		if comma < 0 {
			return nil, apperr.New("INVALID_VERSION", "图片数据格式不正确")
		}
		data = data[comma+1:]
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, apperr.Wrap("INVALID_VERSION", "无法解析图片数据", err)
	}
	if len(raw) == 0 {
		return nil, apperr.New("INVALID_VERSION", "图片数据为空")
	}
	return raw, nil
}

// SaveAssetVersion stores a PNG data URL as the asset's next immutable version.
func (a *App) SaveAssetVersion(assetID, dataURL string) (asset project.Asset, err error) {
	defer logging.Catch("SaveAssetVersion", &err)
	store := a.currentStore()
	if store == nil {
		return project.Asset{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	raw, decodeErr := decodeDataURL(dataURL)
	if decodeErr != nil {
		return project.Asset{}, decodeErr
	}
	return assetlib.SaveVersion(store, assetID, raw)
}

// SaveAssetSheet stores a sprite sheet data URL plus its grid layout. Go
// derives per-frame PNGs, Aseprite/TexturePacker JSON and the thumbnail.
func (a *App) SaveAssetSheet(assetID, dataURL string, info assetlib.AnimationInfo) (asset project.Asset, err error) {
	defer logging.Catch("SaveAssetSheet", &err)
	store := a.currentStore()
	if store == nil {
		return project.Asset{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	raw, decodeErr := decodeDataURL(dataURL)
	if decodeErr != nil {
		return project.Asset{}, decodeErr
	}
	return assetlib.SaveSheet(store, assetID, raw, info)
}

// ExportAsset copies the asset's current version into exports/<名称>/<版本>/.
// Sheets also carry their Aseprite / TexturePacker JSON and derived frames.
func (a *App) ExportAsset(assetID string) (result assetlib.ExportResult, err error) {
	defer logging.Catch("ExportAsset", &err)
	store := a.currentStore()
	if store == nil {
		return assetlib.ExportResult{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return assetlib.ExportAsset(store, assetID)
}

// OpenExportsFolder reveals the project's exports directory in Explorer.
func (a *App) OpenExportsFolder() (err error) {
	defer logging.Catch("OpenExportsFolder", &err)
	store := a.currentStore()
	if store == nil {
		return apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	dir := project.ExportsDir(store.Root())
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return apperr.Wrap("IO", "无法创建导出目录", mkErr)
	}
	return platform.OpenPath(dir)
}

// GeneratedBuilding is one procedural preview sent to the generate screen.
type GeneratedBuilding struct {
	Seed      int64           `json:"seed"`
	Width     int             `json:"width"`
	Height    int             `json:"height"`
	Pivot     project.Point   `json:"pivot"`
	Collision generators.Rect `json:"collision"`
	Image     string          `json:"image"`
}

// GenerateBuildings draws procedural building drafts as PNG data URLs.
func (a *App) GenerateBuildings(opts generators.BuildingOptions) (buildings []GeneratedBuilding, err error) {
	defer logging.Catch("GenerateBuildings", &err)
	if a.currentStore() == nil {
		return nil, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	results, genErr := generators.GenerateBuildings(opts)
	if genErr != nil {
		return nil, apperr.Wrap("GENERATION_FAILED", "生成失败", genErr)
	}
	out := make([]GeneratedBuilding, 0, len(results))
	for _, item := range results {
		out = append(out, GeneratedBuilding{
			Seed:      item.Seed,
			Width:     item.Width,
			Height:    item.Height,
			Pivot:     project.Point{X: item.Pivot.X, Y: item.Pivot.Y},
			Collision: item.Collision,
			Image:     "data:image/png;base64," + base64.StdEncoding.EncodeToString(item.PNG),
		})
	}
	return out, nil
}

// AdoptBuilding persists one generated variant as a draft building asset.
func (a *App) AdoptBuilding(opts generators.BuildingOptions, seed int64, name string) (asset project.Asset, err error) {
	defer logging.Catch("AdoptBuilding", &err)
	store := a.currentStore()
	if store == nil {
		return project.Asset{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	opts.Seed = seed
	opts.Variants = 1
	results, genErr := generators.GenerateBuildings(opts)
	if genErr != nil {
		return project.Asset{}, apperr.Wrap("GENERATION_FAILED", "生成失败", genErr)
	}
	if len(results) == 0 {
		return project.Asset{}, apperr.New("GENERATION_FAILED", "生成结果为空")
	}
	item := results[0]
	label := strings.TrimSpace(name)
	if label == "" {
		label = fmt.Sprintf("建筑 %d", seed)
	}
	return assetlib.CreateGeneratedAsset(store, assetlib.GeneratedInput{
		Name:      label,
		Type:      project.AssetBuilding,
		PaletteID: store.Session().Project.Settings.DefaultPaletteID,
		Width:     item.Width,
		Height:    item.Height,
		Pivot:     project.Point{X: item.Pivot.X, Y: item.Pivot.Y},
		PNG:       item.PNG,
		BasePNG:   item.BasePNG,
		RoofPNG:   item.RoofPNG,
		Generator: "building-v1",
		Seed:      item.Seed,
		Options:   opts,
		Collision: assetlib.AnimationRect{
			X: item.Collision.X,
			Y: item.Collision.Y,
			W: item.Collision.W,
			H: item.Collision.H,
		},
	})
}

// PostProcessPreview is the A/B result of the pixel-perfect chain.
type PostProcessPreview struct {
	Image  string               `json:"image"`
	Width  int                  `json:"width"`
	Height int                  `json:"height"`
	Colors []string             `json:"colors"`
	Seam   imageproc.SeamReport `json:"seam"`
}

// PreviewPostProcess runs the chain on the asset's current image without
// writing anything.
func (a *App) PreviewPostProcess(assetID string, opts imageproc.Options) (preview PostProcessPreview, err error) {
	defer logging.Catch("PreviewPostProcess", &err)
	store := a.currentStore()
	if store == nil {
		return PostProcessPreview{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	result, err := assetlib.ProcessAsset(store, assetID, opts)
	if err != nil {
		return PostProcessPreview{}, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, result.Image); err != nil {
		return PostProcessPreview{}, apperr.Wrap("IO", "无法编码处理结果", err)
	}
	return PostProcessPreview{
		Image:  "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()),
		Width:  result.Width,
		Height: result.Height,
		Colors: result.Colors,
		Seam:   result.Seam,
	}, nil
}

// SavePostProcessedVersion runs the chain and stores it as the next immutable
// version of the asset.
func (a *App) SavePostProcessedVersion(assetID string, opts imageproc.Options) (asset project.Asset, err error) {
	defer logging.Catch("SavePostProcessedVersion", &err)
	store := a.currentStore()
	if store == nil {
		return project.Asset{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	saved, _, err := assetlib.SaveProcessedAsset(store, assetID, opts)
	return saved, err
}

// GetAISettings returns the masked AI provider settings.
func (a *App) GetAISettings() (settings generate.SettingsView, err error) {
	defer logging.Catch("GetAISettings", &err)
	return generate.View(generate.LoadSettings()), nil
}

// GetEditorSettings returns the persisted pixel-editor preferences.
func (a *App) GetEditorSettings() editorprefs.Settings {
	return editorprefs.Load()
}

// SaveEditorSettings persists the pixel-editor preferences.
func (a *App) SaveEditorSettings(settings editorprefs.Settings) (saved editorprefs.Settings, err error) {
	defer logging.Catch("SaveEditorSettings", &err)
	return editorprefs.Save(settings)
}

// SaveAISettings writes the AI form; API keys are encrypted with DPAPI.
func (a *App) SaveAISettings(input generate.SettingsInput) (view generate.SettingsView, err error) {
	defer logging.Catch("SaveAISettings", &err)
	return generate.SaveSettings(input)
}

// TestAIProvider checks one provider without spending a generation call.
func (a *App) TestAIProvider(id string) (result generate.TestResult, err error) {
	defer logging.Catch("TestAIProvider", &err)
	provider, err := generate.NewProvider(generate.LoadSettings(), id)
	if err != nil {
		return generate.TestResult{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return provider.Test(ctx)
}

// GenerateText drafts copy or an image prompt through the chat endpoint.
// It reads the stored settings per call, so a saved text model applies at once.
func (a *App) GenerateText(req generate.TextRequest) (result generate.TextResult, err error) {
	defer logging.Catch("GenerateText", &err)
	return generate.GenerateText(generate.LoadSettings(), req)
}

// SubmitAIJob queues an AI draft request and returns the queued job.
func (a *App) SubmitAIJob(req generate.JobRequest) (job generate.Job, err error) {
	defer logging.Catch("SubmitAIJob", &err)
	store := a.currentStore()
	if store == nil {
		return generate.Job{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	return a.gen.Submit(store.Root(), req)
}

// CancelAIJob stops a queued or running job.
func (a *App) CancelAIJob(id string) (job generate.Job, err error) {
	defer logging.Catch("CancelAIJob", &err)
	return a.gen.Cancel(id)
}

// DeleteAIJob removes a finished, failed or canceled job and its generated
// images. Adopted assets keep their own copies, so they are not affected.
func (a *App) DeleteAIJob(id string) (err error) {
	defer logging.Catch("DeleteAIJob", &err)
	if a.gen == nil {
		return apperr.New("JOB_NOT_FOUND", "任务不存在")
	}
	return a.gen.Delete(id)
}

// ListAIJobs returns this project's generation queue.
func (a *App) ListAIJobs() (jobs []generate.Job, err error) {
	defer logging.Catch("ListAIJobs", &err)
	if a.gen == nil {
		return []generate.Job{}, nil
	}
	return a.gen.List(), nil
}

func validAssetType(value string) string {
	switch value {
	case project.AssetSprite, project.AssetBuilding, project.AssetTile, project.AssetProp,
		project.AssetUI, project.AssetFX, project.AssetOther:
		return value
	default:
		return project.AssetSprite
	}
}

// AdoptAIJobImage stores one generated result as a draft asset.
func (a *App) AdoptAIJobImage(jobID string, index int, name string) (asset project.Asset, err error) {
	defer logging.Catch("AdoptAIJobImage", &err)
	store := a.currentStore()
	if store == nil {
		return project.Asset{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	var job *generate.Job
	for _, candidate := range a.gen.List() {
		if candidate.ID == jobID {
			copied := candidate
			job = &copied
			break
		}
	}
	if job == nil {
		return project.Asset{}, apperr.New("JOB_NOT_FOUND", "任务不存在").WithDetail(jobID)
	}
	if job.State != generate.JobDone {
		return project.Asset{}, apperr.New("JOB_NOT_READY", "任务尚未完成，不能采用")
	}
	if index < 0 || index >= len(job.Images) {
		return project.Asset{}, apperr.New("JOB_NOT_READY", "生成结果序号超出范围")
	}
	image := job.Images[index]
	if image.AdoptedAssetID != "" {
		if existing, getErr := store.GetAsset(image.AdoptedAssetID); getErr == nil {
			return existing, nil
		}
	}
	abs, err := generate.GeneratedFile(store.Root(), jobID, image.File)
	if err != nil {
		return project.Asset{}, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return project.Asset{}, apperr.Wrap("IO", "无法读取生成结果", err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return project.Asset{}, apperr.New("INVALID_GENERATED", "生成结果图片无效")
	}
	label := strings.TrimSpace(name)
	if label == "" {
		label = strings.TrimSpace(job.Name)
	}
	if label == "" {
		label = fmt.Sprintf("AI 素材 %d", index+1)
	}
	created, err := assetlib.CreateGeneratedAsset(store, assetlib.GeneratedInput{
		Name:      label,
		Type:      validAssetType(job.AssetType),
		PaletteID: store.Session().Project.Settings.DefaultPaletteID,
		Width:     config.Width,
		Height:    config.Height,
		Pivot:     project.Point{X: config.Width / 2, Y: config.Height - 1},
		PNG:       data,
		Generator: "ai-" + job.Provider,
		Provider:  job.Provider,
		Seed:      job.Seed,
		Options: map[string]any{
			"jobId":          job.ID,
			"prompt":         job.Prompt,
			"negativePrompt": job.NegativePrompt,
			"width":          job.Width,
			"height":         job.Height,
			"index":          index,
		},
		Collision: assetlib.AnimationRect{X: 0, Y: 0, W: config.Width, H: config.Height},
	})
	if err != nil {
		return project.Asset{}, err
	}
	_, _ = a.gen.MarkAdopted(job.ID, index, created.ID)
	return created, nil
}

// AISheetRequest adopts one AI image as a cleaned directional or FX sheet.
type AISheetRequest struct {
	JobID         string   `json:"jobId"`
	Index         int      `json:"index"`
	Name          string   `json:"name"`
	AssetType     string   `json:"assetType"`
	Columns       int      `json:"columns"`
	Rows          int      `json:"rows"`
	FrameSize     int      `json:"frameSize"`
	FPS           int      `json:"fps"`
	Tags          []string `json:"tags"`
	PaletteID     string   `json:"paletteId"`
	MaxColors     int      `json:"maxColors"`
	Outline       bool     `json:"outline"`
	KeyBackground bool     `json:"keyBackground"`
}

// AdoptAIJobSheet cuts a finished AI image into a grid, cleans every frame with
// the pixel-perfect chain and stores it as an animation asset. Ideal for 4/8
// directional turnarounds and effect strips that the provider returned in one
// image.
func (a *App) AdoptAIJobSheet(req AISheetRequest) (asset project.Asset, err error) {
	defer logging.Catch("AdoptAIJobSheet", &err)
	store := a.currentStore()
	if store == nil {
		return project.Asset{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	var job *generate.Job
	for _, candidate := range a.gen.List() {
		if candidate.ID == req.JobID {
			copied := candidate
			job = &copied
			break
		}
	}
	if job == nil {
		return project.Asset{}, apperr.New("JOB_NOT_FOUND", "任务不存在").WithDetail(req.JobID)
	}
	if job.State != generate.JobDone {
		return project.Asset{}, apperr.New("JOB_NOT_READY", "任务尚未完成，不能采用")
	}
	if req.Index < 0 || req.Index >= len(job.Images) {
		return project.Asset{}, apperr.New("JOB_NOT_READY", "生成结果序号超出范围")
	}
	image := job.Images[req.Index]
	abs, err := generate.GeneratedFile(store.Root(), job.ID, image.File)
	if err != nil {
		return project.Asset{}, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return project.Asset{}, apperr.Wrap("IO", "无法读取生成结果", err)
	}
	label := strings.TrimSpace(req.Name)
	if label == "" {
		label = strings.TrimSpace(job.Name)
	}
	if label == "" {
		label = "AI 精灵表"
	}
	assetType := validAssetType(req.AssetType)
	if req.AssetType == "" {
		assetType = validAssetType(job.AssetType)
	}
	paletteID := req.PaletteID
	if paletteID == "" {
		paletteID = store.Session().Project.Settings.DefaultPaletteID
	}
	return assetlib.CreateAISheet(store, data, assetlib.AISheetOptions{
		Name:          label,
		AssetType:     assetType,
		Columns:       req.Columns,
		Rows:          req.Rows,
		FrameSize:     req.FrameSize,
		FPS:           req.FPS,
		Tags:          req.Tags,
		PaletteID:     paletteID,
		MaxColors:     req.MaxColors,
		Outline:       req.Outline,
		KeyBackground: req.KeyBackground,
		Provider:      job.Provider,
		Seed:          job.Seed,
	})
}

// MapExpandRequest is the per-side tile growth for ExpandMap.
type MapExpandRequest struct {
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
	Left   int `json:"left"`
}

// ExpandMap grows the canvas of one map in place.
func (a *App) ExpandMap(id string, req MapExpandRequest) (mapped project.GameMap, err error) {
	defer logging.Catch("ExpandMap", &err)
	store := a.currentStore()
	if store == nil {
		return project.GameMap{}, apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	current, err := store.LoadMap(id)
	if err != nil {
		return project.GameMap{}, err
	}
	if err := project.ExpandMap(&current, req.Top, req.Right, req.Bottom, req.Left); err != nil {
		return project.GameMap{}, err
	}
	return store.SaveMap(current)
}

// GetAssetURL returns a token-protected URL for a file inside an asset.
func (a *App) GetAssetURL(assetID, file string) (assetURL string, err error) {
	defer logging.Catch("GetAssetURL", &err)
	store := a.currentStore()
	if store == nil {
		return "", apperr.New("NO_PROJECT", "当前没有打开的项目")
	}
	abs, err := store.AssetPath(assetID, file)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return "", apperr.New("ASSET_NOT_FOUND", "素材文件不存在").WithDetail(file)
	}
	base := filepath.Join(store.Root(), "assets", assetID)
	rel, err := filepath.Rel(base, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", apperr.New("INVALID_PATH", "素材文件路径不合法")
	}
	rel = filepath.ToSlash(rel)
	return fmt.Sprintf("/pb-assets/%s/assets/%s/%s?v=%d",
		a.assetToken, url.PathEscape(assetID), escapePathSegments(rel), info.ModTime().Unix()), nil
}

// AssetHandler serves project images to the webview without base64.
func (a *App) AssetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		store := a.currentStore()
		if store == nil {
			http.NotFound(w, r)
			return
		}
		assetPrefix := "/pb-assets/" + a.assetToken + "/"
		genPrefix := "/pb-gen/" + a.assetToken + "/"
		var abs string
		switch {
		case strings.HasPrefix(r.URL.Path, assetPrefix):
			rest := strings.TrimPrefix(r.URL.Path, assetPrefix)
			if !strings.HasPrefix(rest, "assets/") {
				http.NotFound(w, r)
				return
			}
			rel := path.Clean(rest)
			if rel == ".." || strings.HasPrefix(rel, "../") {
				http.NotFound(w, r)
				return
			}
			abs = filepath.Join(store.Root(), filepath.FromSlash(rel))
			if !fsutil.Within(store.Root(), abs) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		case strings.HasPrefix(r.URL.Path, genPrefix):
			rest := strings.TrimPrefix(r.URL.Path, genPrefix)
			parts := strings.Split(rest, "/")
			if len(parts) != 2 {
				http.NotFound(w, r)
				return
			}
			jobID, err := url.PathUnescape(parts[0])
			if err != nil {
				http.NotFound(w, r)
				return
			}
			file, err := url.PathUnescape(parts[1])
			if err != nil {
				http.NotFound(w, r)
				return
			}
			resolved, err := generate.GeneratedFile(store.Root(), jobID, file)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			abs = resolved
		default:
			http.NotFound(w, r)
			return
		}
		info, err := os.Stat(abs)
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "private, max-age=60")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if kind := mime.TypeByExtension(filepath.Ext(abs)); kind != "" {
			w.Header().Set("Content-Type", kind)
		}
		http.ServeFile(w, r, abs)
	})
}

func escapePathSegments(rel string) string {
	parts := strings.Split(rel, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}
