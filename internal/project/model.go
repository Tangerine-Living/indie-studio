package project

import "time"

// SchemaVersion is the current on-disk project schema. It is independent of
// the application version.
const SchemaVersion = 1

// Quest statuses.
const (
	QuestBacklog = "backlog"
	QuestTodo    = "todo"
	QuestDoing   = "doing"
	QuestReview  = "review"
	QuestDone    = "done"
)

// Quest types.
const (
	QuestTypeAsset     = "asset"
	QuestTypeCode      = "code"
	QuestTypeDesign    = "design"
	QuestTypeAudio     = "audio"
	QuestTypeMarketing = "marketing"
)

// Priorities.
const (
	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"
)

// Asset statuses.
const (
	AssetDraft      = "draft"
	AssetReview     = "review"
	AssetApproved   = "approved"
	AssetDeprecated = "deprecated"
)

// Asset types.
const (
	AssetSprite   = "sprite"
	AssetBuilding = "building"
	AssetTile     = "tile"
	AssetProp     = "prop"
	AssetUI       = "ui"
	AssetFX       = "fx"
	AssetOther    = "other"
)

// Production stages shown on the dashboard.
const (
	StageConcept    = "concept"
	StagePrototype  = "prototype"
	StageProduction = "production"
	StagePolish     = "polish"
	StageRelease    = "release"
)

// Project is the content of project.json.
type Project struct {
	SchemaVersion int         `json:"schemaVersion"`
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
	Engine        string      `json:"engine"`
	Stage         string      `json:"stage"`
	CoverAssetID  string      `json:"coverAssetId,omitempty"`
	Settings      Settings    `json:"settings"`
	Members       []Member    `json:"members"`
	Milestones    []Milestone `json:"milestones"`
}

// Settings holds project-wide defaults.
type Settings struct {
	TileSize         int        `json:"tileSize"`
	DefaultPaletteID string     `json:"defaultPaletteId"`
	PixelScale       int        `json:"pixelScale"`
	UILanguage       string     `json:"uiLanguage"`
	Template         string     `json:"template,omitempty"`
	StyleGuide       StyleGuide `json:"styleGuide"`
}

// StyleGuide is injected into AI prompts and used by the post-processing
// pipeline.
type StyleGuide struct {
	Perspective    string `json:"perspective"`
	Outline        string `json:"outline"`
	Light          string `json:"light"`
	MaxColors      int    `json:"maxColors"`
	NegativePrompt string `json:"negativePrompt,omitempty"`
	// StylePrompt is a user-written anchor appended to every AI draft prompt.
	StylePrompt string `json:"stylePrompt,omitempty"`
}

// Member is a local, account-free project member.
type Member struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// Milestone is a project-level delivery target.
type Milestone struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	DueAt string `json:"dueAt,omitempty"`
	Done  bool   `json:"done"`
}

// Session is a project plus its on-disk path.
type Session struct {
	Path    string  `json:"path"`
	Project Project `json:"project"`
}

// RecentProject is one entry in the global recent-projects list.
type RecentProject struct {
	Path         string    `json:"path"`
	Name         string    `json:"name"`
	LastOpenedAt time.Time `json:"lastOpenedAt"`
}

// CreateOptions is the request from the new-project wizard.
type CreateOptions struct {
	ParentDir string `json:"parentDir"`
	Name      string `json:"name"`
	Engine    string `json:"engine"`
	TileSize  int    `json:"tileSize"`
	Template  string `json:"template"`
	PaletteID string `json:"paletteId"`
}

// Quest is a task or commission in the project. Quests can carry an asset
// brief and link to generated assets.
type Quest struct {
	SchemaVersion int        `json:"schemaVersion"`
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Type          string     `json:"type"`
	Status        string     `json:"status"`
	Priority      string     `json:"priority"`
	Assignee      string     `json:"assignee,omitempty"`
	StartAt       string     `json:"startAt,omitempty"`
	DueAt         string     `json:"dueAt,omitempty"`
	Description   string     `json:"description,omitempty"`
	Brief         QuestBrief `json:"brief"`
	Acceptance    []string   `json:"acceptance"`
	Links         QuestLinks `json:"links"`
	Comments      []Comment  `json:"comments"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// QuestBrief is the asset brief that seeds a generation job.
type QuestBrief struct {
	AssetType  string   `json:"assetType"`
	TileSize   int      `json:"tileSize"`
	PaletteID  string   `json:"paletteId"`
	Style      string   `json:"style"`
	References []string `json:"references"`
	Prompt     string   `json:"prompt"`
}

// QuestLinks connects a quest to produced content.
type QuestLinks struct {
	AssetIDs []string `json:"assetIds"`
	MapIDs   []string `json:"mapIds"`
	URLs     []string `json:"urls"`
}

// Comment is a local comment on a quest.
type Comment struct {
	At     time.Time `json:"at"`
	Author string    `json:"author"`
	Text   string    `json:"text"`
}

// Asset is one entry in the asset library. The actual files live beside
// asset.json.
type Asset struct {
	SchemaVersion  int            `json:"schemaVersion"`
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Type           string         `json:"type"`
	Tags           []string       `json:"tags"`
	Status         string         `json:"status"`
	Dimensions     Dimensions     `json:"dimensions"`
	Pivot          Point          `json:"pivot"`
	PaletteID      string         `json:"paletteId"`
	QuestIDs       []string       `json:"questIds"`
	License        string         `json:"license"`
	Source         AssetSource    `json:"source"`
	SourceHash     string         `json:"sourceHash,omitempty"`
	CurrentVersion string         `json:"currentVersion,omitempty"`
	Versions       []AssetVersion `json:"versions"`
	Animation      *Animation     `json:"animation,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// Dimensions is a pixel size.
type Dimensions struct {
	W int `json:"w"`
	H int `json:"h"`
}

// Point is a pivot or anchor in pixels.
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// AssetSource records where an asset came from.
type AssetSource struct {
	Kind     string `json:"kind"` // imported | generated | procedural
	Provider string `json:"provider,omitempty"`
	Workflow string `json:"workflow,omitempty"`
}

// AssetVersion is an immutable generation or edit result.
type AssetVersion struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	Hash      string    `json:"hash"`
	MetaFile  string    `json:"metaFile"`
	Size      int64     `json:"size"`
}

// Animation describes an asset's frame sequence and sheet layout.
type Animation struct {
	FPS         int      `json:"fps"`
	Loop        bool     `json:"loop"`
	Tags        []string `json:"tags"`
	FrameWidth  int      `json:"frameWidth,omitempty"`
	FrameHeight int      `json:"frameHeight,omitempty"`
	FrameCount  int      `json:"frameCount,omitempty"`
	Columns     int      `json:"columns,omitempty"`
	// FrameDurations is the per-frame hold time in milliseconds. When empty the
	// uniform duration derived from FPS is used.
	FrameDurations []int `json:"frameDurations,omitempty"`
}

// Palette is a fixed colour palette.
type Palette struct {
	SchemaVersion int      `json:"schemaVersion"`
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Colors        []string `json:"colors"`
	Source        string   `json:"source,omitempty"`
	Locked        bool     `json:"locked"`
}

// Map layer types. Tile layers carry sparse tiles; the object layer holds
// objects, and collision tiles mark solid ground.
const (
	MapGround      = "ground"
	MapDecoration  = "decoration"
	MapBuilding    = "building"
	MapRoof        = "roof"
	MapLayerObject = "object"
	MapCollision   = "collision"
	MapLight       = "light"
)

// Map object kinds.
const (
	MapObjectSpawn    = "spawn"
	MapObjectNPC      = "npc"
	MapObjectChest    = "chest"
	MapObjectTrigger  = "trigger"
	MapObjectBuilding = "building"
	MapObjectProp     = "prop"
)

// GameMap is one 2.5D top-down map stored as maps/<id>.map.json.
type GameMap struct {
	SchemaVersion int          `json:"schemaVersion"`
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	TileSize      int          `json:"tileSize"`
	Width         int          `json:"width"`
	Height        int          `json:"height"`
	Tilesets      []MapTileset `json:"tilesets"`
	Layers        []MapLayer   `json:"layers"`
	Objects       []MapObject  `json:"objects"`
	Settings      MapSettings  `json:"settings"`
	CreatedAt     time.Time    `json:"createdAt"`
	UpdatedAt     time.Time    `json:"updatedAt"`
}

// MapSummary is the list payload; full tile data is loaded on demand.
type MapSummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	TileSize  int       `json:"tileSize"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// MapTileset points at one asset atlas used by the map.
type MapTileset struct {
	ID        string `json:"id"`
	AssetID   string `json:"assetId"`
	TileSize  int    `json:"tileSize"`
	Columns   int    `json:"columns"`
	TileCount int    `json:"tileCount"`
}

// MapLayer is one tile or metadata layer. Tile positions are sparse: only
// painted cells are stored, which keeps git diffs readable.
type MapLayer struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Type      string       `json:"type"`
	Elevation int          `json:"elevation"`
	Visible   bool         `json:"visible"`
	Opacity   float64      `json:"opacity"`
	Autotile  *MapAutotile `json:"autotile,omitempty"`
	Tiles     []MapTile    `json:"tiles"`
}

// MapAutotile records the variant family a terrain layer paints with.
type MapAutotile struct {
	Set  int    `json:"set"`
	Base int    `json:"base"`
	Mode string `json:"mode"` // 16 | 47
}

// MapTile is one painted cell. Set indexes Tilesets; Tile is 0-based.
type MapTile struct {
	X      int  `json:"x"`
	Y      int  `json:"y"`
	Set    int  `json:"set"`
	Tile   int  `json:"tile"`
	FlipH  bool `json:"flipH,omitempty"`
	FlipV  bool `json:"flipV,omitempty"`
	Rotate int  `json:"rotate,omitempty"` // 0 | 90 | 180 | 270
}

// MapObject is a spawn point, NPC, chest, trigger, building or prop.
type MapObject struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Kind       string            `json:"kind"`
	X          float64           `json:"x"` // tile units, top-left
	Y          float64           `json:"y"`
	W          float64           `json:"w"`
	H          float64           `json:"h"`
	Elevation  int               `json:"elevation"`
	YSort      bool              `json:"ySort"`
	RoofFade   bool              `json:"roofFade"`
	Solid      bool              `json:"solid"`
	AssetID    string            `json:"assetId,omitempty"`
	Color      string            `json:"color,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
}

// MapSettings holds map-wide rendering and export defaults.
type MapSettings struct {
	YSort      bool   `json:"ySort"`
	RoofFade   bool   `json:"roofFade"`
	HeightStep int    `json:"heightStep"`
	Ambient    string `json:"ambient"`
}

// Entity kinds in the setting library.
const (
	EntityCharacter = "character"
	EntityPlace     = "place"
	EntityFaction   = "faction"
	EntityItem      = "item"
	EntityEvent     = "event"
	EntityTerm      = "term"
)

// Entity statuses describe how settled a piece of setting is.
const (
	EntityIdea   = "idea"
	EntityDraft  = "draft"
	EntityStable = "stable"
)

// Entity is one piece of worldbuilding: a character, place, faction, item,
// event or term. Entities reference each other by ID, and link the assets and
// maps that depict them.
type Entity struct {
	SchemaVersion int              `json:"schemaVersion"`
	ID            string           `json:"id"`
	Kind          string           `json:"kind"`
	Name          string           `json:"name"`
	Status        string           `json:"status"`
	Aliases       []string         `json:"aliases"`
	Summary       string           `json:"summary"`
	Body          string           `json:"body"`
	Tags          []string         `json:"tags"`
	Fields        []EntityField    `json:"fields"`
	Relations     []EntityRelation `json:"relations"`
	AssetIDs      []string         `json:"assetIds"`
	MapIDs        []string         `json:"mapIds"`
	// ArtPrompt is the user-confirmed prompt reused for every generation of
	// this entry, keeping the character's look stable across runs.
	ArtPrompt string    `json:"artPrompt,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// EntityField is a free-form key/value fact, such as age or capital.
type EntityField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// EntityRelation points at another entity. Label is the relationship as the
// author writes it ("父亲", "效忠于", "位于"), so no fixed vocabulary is needed.
type EntityRelation struct {
	EntityID string `json:"entityId"`
	Label    string `json:"label"`
}
