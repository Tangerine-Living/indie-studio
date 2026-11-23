export namespace assets {
	
	export class AnimationInfo {
	    frameWidth: number;
	    frameHeight: number;
	    frameCount: number;
	    columns: number;
	    rows: number;
	    margin: number;
	    spacing: number;
	    fps: number;
	    loop: boolean;
	    durations?: number[];
	    tags?: string[];
	
	    static createFrom(source: any = {}) {
	        return new AnimationInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.frameWidth = source["frameWidth"];
	        this.frameHeight = source["frameHeight"];
	        this.frameCount = source["frameCount"];
	        this.columns = source["columns"];
	        this.rows = source["rows"];
	        this.margin = source["margin"];
	        this.spacing = source["spacing"];
	        this.fps = source["fps"];
	        this.loop = source["loop"];
	        this.durations = source["durations"];
	        this.tags = source["tags"];
	    }
	}
	export class BlankAssetInput {
	    name: string;
	    type: string;
	    width: number;
	    height: number;
	    paletteId: string;
	
	    static createFrom(source: any = {}) {
	        return new BlankAssetInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.paletteId = source["paletteId"];
	    }
	}
	export class Duplicate {
	    path: string;
	    assetId: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new Duplicate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.assetId = source["assetId"];
	        this.name = source["name"];
	    }
	}
	export class ExportResult {
	    dir: string;
	    files: string[];
	
	    static createFrom(source: any = {}) {
	        return new ExportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dir = source["dir"];
	        this.files = source["files"];
	    }
	}
	export class Failure {
	    path: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Failure(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.message = source["message"];
	    }
	}
	export class Summary {
	    imported: project.Asset[];
	    duplicates: Duplicate[];
	    failed: Failure[];
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imported = this.convertValues(source["imported"], project.Asset);
	        this.duplicates = this.convertValues(source["duplicates"], Duplicate);
	        this.failed = this.convertValues(source["failed"], Failure);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace editorprefs {
	
	export class Settings {
	    showGrid: boolean;
	    openMode: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.showGrid = source["showGrid"];
	        this.openMode = source["openMode"];
	    }
	}

}

export namespace export {
	
	export class MapExportResult {
	    dir: string;
	    files: string[];
	    format: string;
	
	    static createFrom(source: any = {}) {
	        return new MapExportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dir = source["dir"];
	        this.files = source["files"];
	        this.format = source["format"];
	    }
	}
	export class TableExportResult {
	    dir: string;
	    files: string[];
	
	    static createFrom(source: any = {}) {
	        return new TableExportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dir = source["dir"];
	        this.files = source["files"];
	    }
	}

}

export namespace generate {
	
	export class ComfyConfig {
	    baseUrl: string;
	    workflowName: string;
	    workflow: string;
	
	    static createFrom(source: any = {}) {
	        return new ComfyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.baseUrl = source["baseUrl"];
	        this.workflowName = source["workflowName"];
	        this.workflow = source["workflow"];
	    }
	}
	export class JobImage {
	    index: number;
	    file: string;
	    url: string;
	    width: number;
	    height: number;
	    adoptedAssetId?: string;
	
	    static createFrom(source: any = {}) {
	        return new JobImage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.file = source["file"];
	        this.url = source["url"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.adoptedAssetId = source["adoptedAssetId"];
	    }
	}
	export class Job {
	    id: string;
	    provider: string;
	    state: string;
	    progress: number;
	    stage: string;
	    prompt: string;
	    negativePrompt: string;
	    width: number;
	    height: number;
	    count: number;
	    seed: number;
	    assetType: string;
	    name: string;
	    mode?: string;
	    targetAssetId?: string;
	    baseImage?: string;
	    mask?: string;
	    error?: string;
	    images: JobImage[];
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.provider = source["provider"];
	        this.state = source["state"];
	        this.progress = source["progress"];
	        this.stage = source["stage"];
	        this.prompt = source["prompt"];
	        this.negativePrompt = source["negativePrompt"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.count = source["count"];
	        this.seed = source["seed"];
	        this.assetType = source["assetType"];
	        this.name = source["name"];
	        this.mode = source["mode"];
	        this.targetAssetId = source["targetAssetId"];
	        this.baseImage = source["baseImage"];
	        this.mask = source["mask"];
	        this.error = source["error"];
	        this.images = this.convertValues(source["images"], JobImage);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class JobRequest {
	    provider: string;
	    prompt: string;
	    negativePrompt: string;
	    width: number;
	    height: number;
	    count: number;
	    seed: number;
	    assetType: string;
	    name: string;
	    mode?: string;
	    targetAssetId?: string;
	    baseImage?: string;
	    mask?: string;
	
	    static createFrom(source: any = {}) {
	        return new JobRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.prompt = source["prompt"];
	        this.negativePrompt = source["negativePrompt"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.count = source["count"];
	        this.seed = source["seed"];
	        this.assetType = source["assetType"];
	        this.name = source["name"];
	        this.mode = source["mode"];
	        this.targetAssetId = source["targetAssetId"];
	        this.baseImage = source["baseImage"];
	        this.mask = source["mask"];
	    }
	}
	export class OpenAIInput {
	    baseUrl: string;
	    model: string;
	    editModel?: string;
	    textModel: string;
	    size: string;
	    apiKey: string;
	    clearKey: boolean;
	    textProvider?: string;
	    textBaseUrl?: string;
	    textApiKey?: string;
	    textClearKey?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OpenAIInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.baseUrl = source["baseUrl"];
	        this.model = source["model"];
	        this.editModel = source["editModel"];
	        this.textModel = source["textModel"];
	        this.size = source["size"];
	        this.apiKey = source["apiKey"];
	        this.clearKey = source["clearKey"];
	        this.textProvider = source["textProvider"];
	        this.textBaseUrl = source["textBaseUrl"];
	        this.textApiKey = source["textApiKey"];
	        this.textClearKey = source["textClearKey"];
	    }
	}
	export class OpenAIPublic {
	    baseUrl: string;
	    model: string;
	    editModel: string;
	    textModel: string;
	    size: string;
	    hasKey: boolean;
	    textProvider: string;
	    textBaseUrl: string;
	    textHasKey: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OpenAIPublic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.baseUrl = source["baseUrl"];
	        this.model = source["model"];
	        this.editModel = source["editModel"];
	        this.textModel = source["textModel"];
	        this.size = source["size"];
	        this.hasKey = source["hasKey"];
	        this.textProvider = source["textProvider"];
	        this.textBaseUrl = source["textBaseUrl"];
	        this.textHasKey = source["textHasKey"];
	    }
	}
	export class SettingsInput {
	    defaultProvider: string;
	    comfy: ComfyConfig;
	    openai: OpenAIInput;
	
	    static createFrom(source: any = {}) {
	        return new SettingsInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultProvider = source["defaultProvider"];
	        this.comfy = this.convertValues(source["comfy"], ComfyConfig);
	        this.openai = this.convertValues(source["openai"], OpenAIInput);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SettingsView {
	    defaultProvider: string;
	    comfy: ComfyConfig;
	    openai: OpenAIPublic;
	
	    static createFrom(source: any = {}) {
	        return new SettingsView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultProvider = source["defaultProvider"];
	        this.comfy = this.convertValues(source["comfy"], ComfyConfig);
	        this.openai = this.convertValues(source["openai"], OpenAIPublic);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TestResult {
	    ok: boolean;
	    message: string;
	    detail?: string;
	    text?: TestResult;
	
	    static createFrom(source: any = {}) {
	        return new TestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.message = source["message"];
	        this.detail = source["detail"];
	        this.text = this.convertValues(source["text"], TestResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TextRequest {
	    system: string;
	    user: string;
	    maxTokens?: number;
	    temperature?: number;
	
	    static createFrom(source: any = {}) {
	        return new TextRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.system = source["system"];
	        this.user = source["user"];
	        this.maxTokens = source["maxTokens"];
	        this.temperature = source["temperature"];
	    }
	}
	export class TextResult {
	    text: string;
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new TextResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.model = source["model"];
	    }
	}

}

export namespace generators {
	
	export class BuildingOptions {
	    seed: number;
	    width: number;
	    depth: number;
	    tileSize: number;
	    floors: number;
	    wallColor: string;
	    roofColor: string;
	    trimColor: string;
	    doorColor: string;
	    windows: number;
	    sign: boolean;
	    weathered: number;
	    shadow: boolean;
	    outline: boolean;
	    variants: number;
	
	    static createFrom(source: any = {}) {
	        return new BuildingOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.seed = source["seed"];
	        this.width = source["width"];
	        this.depth = source["depth"];
	        this.tileSize = source["tileSize"];
	        this.floors = source["floors"];
	        this.wallColor = source["wallColor"];
	        this.roofColor = source["roofColor"];
	        this.trimColor = source["trimColor"];
	        this.doorColor = source["doorColor"];
	        this.windows = source["windows"];
	        this.sign = source["sign"];
	        this.weathered = source["weathered"];
	        this.shadow = source["shadow"];
	        this.outline = source["outline"];
	        this.variants = source["variants"];
	    }
	}
	export class Rect {
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	
	    static createFrom(source: any = {}) {
	        return new Rect(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	    }
	}

}

export namespace imageproc {
	
	export class Options {
	    width: number;
	    height: number;
	    trim: boolean;
	    alphaThreshold: number;
	    maxColors: number;
	    colors: string[];
	    dither: string;
	    ditherStrength: number;
	    cleanup: boolean;
	    outline: boolean;
	    outlineColor: string;
	    checkSeam: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Options(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.width = source["width"];
	        this.height = source["height"];
	        this.trim = source["trim"];
	        this.alphaThreshold = source["alphaThreshold"];
	        this.maxColors = source["maxColors"];
	        this.colors = source["colors"];
	        this.dither = source["dither"];
	        this.ditherStrength = source["ditherStrength"];
	        this.cleanup = source["cleanup"];
	        this.outline = source["outline"];
	        this.outlineColor = source["outlineColor"];
	        this.checkSeam = source["checkSeam"];
	    }
	}
	export class SeamReport {
	    leftRight: number;
	    topBottom: number;
	
	    static createFrom(source: any = {}) {
	        return new SeamReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.leftRight = source["leftRight"];
	        this.topBottom = source["topBottom"];
	    }
	}

}

export namespace main {
	
	export class AISheetRequest {
	    jobId: string;
	    index: number;
	    name: string;
	    assetType: string;
	    columns: number;
	    rows: number;
	    frameSize: number;
	    fps: number;
	    tags: string[];
	    paletteId: string;
	    maxColors: number;
	    outline: boolean;
	    keyBackground: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AISheetRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.jobId = source["jobId"];
	        this.index = source["index"];
	        this.name = source["name"];
	        this.assetType = source["assetType"];
	        this.columns = source["columns"];
	        this.rows = source["rows"];
	        this.frameSize = source["frameSize"];
	        this.fps = source["fps"];
	        this.tags = source["tags"];
	        this.paletteId = source["paletteId"];
	        this.maxColors = source["maxColors"];
	        this.outline = source["outline"];
	        this.keyBackground = source["keyBackground"];
	    }
	}
	export class AppInfo {
	    name: string;
	    version: string;
	    platform: string;
	    logPath: string;
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.platform = source["platform"];
	        this.logPath = source["logPath"];
	        this.mode = source["mode"];
	    }
	}
	export class AssetSourceInfo {
	    file: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new AssetSourceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.url = source["url"];
	    }
	}
	export class Bootstrap {
	    info: AppInfo;
	    recent: project.RecentProject[];
	    session?: project.Session;
	
	    static createFrom(source: any = {}) {
	        return new Bootstrap(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.info = this.convertValues(source["info"], AppInfo);
	        this.recent = this.convertValues(source["recent"], project.RecentProject);
	        this.session = this.convertValues(source["session"], project.Session);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GeneratedBuilding {
	    seed: number;
	    width: number;
	    height: number;
	    pivot: project.Point;
	    collision: generators.Rect;
	    image: string;
	
	    static createFrom(source: any = {}) {
	        return new GeneratedBuilding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.seed = source["seed"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.pivot = this.convertValues(source["pivot"], project.Point);
	        this.collision = this.convertValues(source["collision"], generators.Rect);
	        this.image = source["image"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ImportOptions {
	    paths: string[];
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.paths = source["paths"];
	        this.type = source["type"];
	    }
	}
	export class MapExpandRequest {
	    top: number;
	    right: number;
	    bottom: number;
	    left: number;
	
	    static createFrom(source: any = {}) {
	        return new MapExpandRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.top = source["top"];
	        this.right = source["right"];
	        this.bottom = source["bottom"];
	        this.left = source["left"];
	    }
	}
	export class PostProcessPreview {
	    image: string;
	    width: number;
	    height: number;
	    colors: string[];
	    seam: imageproc.SeamReport;
	
	    static createFrom(source: any = {}) {
	        return new PostProcessPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.image = source["image"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.colors = source["colors"];
	        this.seam = this.convertValues(source["seam"], imageproc.SeamReport);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace project {
	
	export class Animation {
	    fps: number;
	    loop: boolean;
	    tags: string[];
	    frameWidth?: number;
	    frameHeight?: number;
	    frameCount?: number;
	    columns?: number;
	    frameDurations?: number[];
	
	    static createFrom(source: any = {}) {
	        return new Animation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fps = source["fps"];
	        this.loop = source["loop"];
	        this.tags = source["tags"];
	        this.frameWidth = source["frameWidth"];
	        this.frameHeight = source["frameHeight"];
	        this.frameCount = source["frameCount"];
	        this.columns = source["columns"];
	        this.frameDurations = source["frameDurations"];
	    }
	}
	export class AssetVersion {
	    id: string;
	    // Go type: time
	    createdAt: any;
	    hash: string;
	    metaFile: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new AssetVersion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.hash = source["hash"];
	        this.metaFile = source["metaFile"];
	        this.size = source["size"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AssetSource {
	    kind: string;
	    provider?: string;
	    workflow?: string;
	
	    static createFrom(source: any = {}) {
	        return new AssetSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.provider = source["provider"];
	        this.workflow = source["workflow"];
	    }
	}
	export class Point {
	    x: number;
	    y: number;
	
	    static createFrom(source: any = {}) {
	        return new Point(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	    }
	}
	export class Dimensions {
	    w: number;
	    h: number;
	
	    static createFrom(source: any = {}) {
	        return new Dimensions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.w = source["w"];
	        this.h = source["h"];
	    }
	}
	export class Asset {
	    schemaVersion: number;
	    id: string;
	    name: string;
	    type: string;
	    tags: string[];
	    status: string;
	    dimensions: Dimensions;
	    pivot: Point;
	    paletteId: string;
	    questIds: string[];
	    license: string;
	    source: AssetSource;
	    sourceHash?: string;
	    currentVersion?: string;
	    versions: AssetVersion[];
	    animation?: Animation;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Asset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.tags = source["tags"];
	        this.status = source["status"];
	        this.dimensions = this.convertValues(source["dimensions"], Dimensions);
	        this.pivot = this.convertValues(source["pivot"], Point);
	        this.paletteId = source["paletteId"];
	        this.questIds = source["questIds"];
	        this.license = source["license"];
	        this.source = this.convertValues(source["source"], AssetSource);
	        this.sourceHash = source["sourceHash"];
	        this.currentVersion = source["currentVersion"];
	        this.versions = this.convertValues(source["versions"], AssetVersion);
	        this.animation = this.convertValues(source["animation"], Animation);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class Comment {
	    // Go type: time
	    at: any;
	    author: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new Comment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.at = this.convertValues(source["at"], null);
	        this.author = source["author"];
	        this.text = source["text"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CreateOptions {
	    parentDir: string;
	    name: string;
	    engine: string;
	    tileSize: number;
	    template: string;
	    paletteId: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.parentDir = source["parentDir"];
	        this.name = source["name"];
	        this.engine = source["engine"];
	        this.tileSize = source["tileSize"];
	        this.template = source["template"];
	        this.paletteId = source["paletteId"];
	    }
	}
	export class TableRow {
	    id: string;
	    values: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new TableRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.values = source["values"];
	    }
	}
	export class TableColumn {
	    id: string;
	    name: string;
	    key: string;
	    type: string;
	    required: boolean;
	    options: string[];
	    refTableId?: string;
	    min?: number;
	    max?: number;
	    note?: string;
	
	    static createFrom(source: any = {}) {
	        return new TableColumn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.key = source["key"];
	        this.type = source["type"];
	        this.required = source["required"];
	        this.options = source["options"];
	        this.refTableId = source["refTableId"];
	        this.min = source["min"];
	        this.max = source["max"];
	        this.note = source["note"];
	    }
	}
	export class DataTable {
	    schemaVersion: number;
	    id: string;
	    name: string;
	    key: string;
	    note: string;
	    columns: TableColumn[];
	    rows: TableRow[];
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new DataTable(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.key = source["key"];
	        this.note = source["note"];
	        this.columns = this.convertValues(source["columns"], TableColumn);
	        this.rows = this.convertValues(source["rows"], TableRow);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class EntityRelation {
	    entityId: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new EntityRelation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entityId = source["entityId"];
	        this.label = source["label"];
	    }
	}
	export class EntityField {
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new EntityField(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class Entity {
	    schemaVersion: number;
	    id: string;
	    kind: string;
	    name: string;
	    status: string;
	    aliases: string[];
	    summary: string;
	    body: string;
	    tags: string[];
	    fields: EntityField[];
	    relations: EntityRelation[];
	    assetIds: string[];
	    mapIds: string[];
	    artPrompt?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Entity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.aliases = source["aliases"];
	        this.summary = source["summary"];
	        this.body = source["body"];
	        this.tags = source["tags"];
	        this.fields = this.convertValues(source["fields"], EntityField);
	        this.relations = this.convertValues(source["relations"], EntityRelation);
	        this.assetIds = source["assetIds"];
	        this.mapIds = source["mapIds"];
	        this.artPrompt = source["artPrompt"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class MapSettings {
	    ySort: boolean;
	    roofFade: boolean;
	    heightStep: number;
	    ambient: string;
	
	    static createFrom(source: any = {}) {
	        return new MapSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ySort = source["ySort"];
	        this.roofFade = source["roofFade"];
	        this.heightStep = source["heightStep"];
	        this.ambient = source["ambient"];
	    }
	}
	export class MapObject {
	    id: string;
	    name: string;
	    kind: string;
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	    elevation: number;
	    ySort: boolean;
	    roofFade: boolean;
	    solid: boolean;
	    assetId?: string;
	    color?: string;
	    properties?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new MapObject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	        this.elevation = source["elevation"];
	        this.ySort = source["ySort"];
	        this.roofFade = source["roofFade"];
	        this.solid = source["solid"];
	        this.assetId = source["assetId"];
	        this.color = source["color"];
	        this.properties = source["properties"];
	    }
	}
	export class MapTile {
	    x: number;
	    y: number;
	    set: number;
	    tile: number;
	    flipH?: boolean;
	    flipV?: boolean;
	    rotate?: number;
	
	    static createFrom(source: any = {}) {
	        return new MapTile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.set = source["set"];
	        this.tile = source["tile"];
	        this.flipH = source["flipH"];
	        this.flipV = source["flipV"];
	        this.rotate = source["rotate"];
	    }
	}
	export class MapAutotile {
	    set: number;
	    base: number;
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new MapAutotile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.set = source["set"];
	        this.base = source["base"];
	        this.mode = source["mode"];
	    }
	}
	export class MapLayer {
	    id: string;
	    name: string;
	    type: string;
	    elevation: number;
	    visible: boolean;
	    opacity: number;
	    autotile?: MapAutotile;
	    tiles: MapTile[];
	
	    static createFrom(source: any = {}) {
	        return new MapLayer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.elevation = source["elevation"];
	        this.visible = source["visible"];
	        this.opacity = source["opacity"];
	        this.autotile = this.convertValues(source["autotile"], MapAutotile);
	        this.tiles = this.convertValues(source["tiles"], MapTile);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MapTileset {
	    id: string;
	    assetId: string;
	    tileSize: number;
	    columns: number;
	    tileCount: number;
	
	    static createFrom(source: any = {}) {
	        return new MapTileset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.assetId = source["assetId"];
	        this.tileSize = source["tileSize"];
	        this.columns = source["columns"];
	        this.tileCount = source["tileCount"];
	    }
	}
	export class GameMap {
	    schemaVersion: number;
	    id: string;
	    name: string;
	    tileSize: number;
	    width: number;
	    height: number;
	    tilesets: MapTileset[];
	    layers: MapLayer[];
	    objects: MapObject[];
	    settings: MapSettings;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new GameMap(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.tileSize = source["tileSize"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.tilesets = this.convertValues(source["tilesets"], MapTileset);
	        this.layers = this.convertValues(source["layers"], MapLayer);
	        this.objects = this.convertValues(source["objects"], MapObject);
	        this.settings = this.convertValues(source["settings"], MapSettings);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	export class MapSummary {
	    id: string;
	    name: string;
	    tileSize: number;
	    width: number;
	    height: number;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new MapSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.tileSize = source["tileSize"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class Member {
	    id: string;
	    name: string;
	    role: string;
	
	    static createFrom(source: any = {}) {
	        return new Member(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.role = source["role"];
	    }
	}
	export class Milestone {
	    id: string;
	    title: string;
	    dueAt?: string;
	    done: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Milestone(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.dueAt = source["dueAt"];
	        this.done = source["done"];
	    }
	}
	export class Palette {
	    schemaVersion: number;
	    id: string;
	    name: string;
	    colors: string[];
	    source?: string;
	    locked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Palette(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.colors = source["colors"];
	        this.source = source["source"];
	        this.locked = source["locked"];
	    }
	}
	
	export class StyleGuide {
	    perspective: string;
	    outline: string;
	    light: string;
	    maxColors: number;
	    negativePrompt?: string;
	    stylePrompt?: string;
	
	    static createFrom(source: any = {}) {
	        return new StyleGuide(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.perspective = source["perspective"];
	        this.outline = source["outline"];
	        this.light = source["light"];
	        this.maxColors = source["maxColors"];
	        this.negativePrompt = source["negativePrompt"];
	        this.stylePrompt = source["stylePrompt"];
	    }
	}
	export class Settings {
	    tileSize: number;
	    defaultPaletteId: string;
	    pixelScale: number;
	    uiLanguage: string;
	    template?: string;
	    styleGuide: StyleGuide;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tileSize = source["tileSize"];
	        this.defaultPaletteId = source["defaultPaletteId"];
	        this.pixelScale = source["pixelScale"];
	        this.uiLanguage = source["uiLanguage"];
	        this.template = source["template"];
	        this.styleGuide = this.convertValues(source["styleGuide"], StyleGuide);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Project {
	    schemaVersion: number;
	    id: string;
	    name: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    engine: string;
	    stage: string;
	    coverAssetId?: string;
	    settings: Settings;
	    members: Member[];
	    milestones: Milestone[];
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.engine = source["engine"];
	        this.stage = source["stage"];
	        this.coverAssetId = source["coverAssetId"];
	        this.settings = this.convertValues(source["settings"], Settings);
	        this.members = this.convertValues(source["members"], Member);
	        this.milestones = this.convertValues(source["milestones"], Milestone);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class QuestLinks {
	    assetIds: string[];
	    mapIds: string[];
	    urls: string[];
	
	    static createFrom(source: any = {}) {
	        return new QuestLinks(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.assetIds = source["assetIds"];
	        this.mapIds = source["mapIds"];
	        this.urls = source["urls"];
	    }
	}
	export class QuestBrief {
	    assetType: string;
	    tileSize: number;
	    paletteId: string;
	    style: string;
	    references: string[];
	    prompt: string;
	
	    static createFrom(source: any = {}) {
	        return new QuestBrief(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.assetType = source["assetType"];
	        this.tileSize = source["tileSize"];
	        this.paletteId = source["paletteId"];
	        this.style = source["style"];
	        this.references = source["references"];
	        this.prompt = source["prompt"];
	    }
	}
	export class Quest {
	    schemaVersion: number;
	    id: string;
	    title: string;
	    type: string;
	    status: string;
	    priority: string;
	    assignee?: string;
	    startAt?: string;
	    dueAt?: string;
	    description?: string;
	    brief: QuestBrief;
	    acceptance: string[];
	    links: QuestLinks;
	    comments: Comment[];
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Quest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.title = source["title"];
	        this.type = source["type"];
	        this.status = source["status"];
	        this.priority = source["priority"];
	        this.assignee = source["assignee"];
	        this.startAt = source["startAt"];
	        this.dueAt = source["dueAt"];
	        this.description = source["description"];
	        this.brief = this.convertValues(source["brief"], QuestBrief);
	        this.acceptance = source["acceptance"];
	        this.links = this.convertValues(source["links"], QuestLinks);
	        this.comments = this.convertValues(source["comments"], Comment);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class RecentProject {
	    path: string;
	    name: string;
	    // Go type: time
	    lastOpenedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new RecentProject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.lastOpenedAt = this.convertValues(source["lastOpenedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Session {
	    path: string;
	    project: Project;
	
	    static createFrom(source: any = {}) {
	        return new Session(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.project = this.convertValues(source["project"], Project);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	

}

export namespace update {
	
	export class Settings {
	    channel: string;
	    autoCheck: boolean;
	    manifestUrl: string;
	    signatureUrl?: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.channel = source["channel"];
	        this.autoCheck = source["autoCheck"];
	        this.manifestUrl = source["manifestUrl"];
	        this.signatureUrl = source["signatureUrl"];
	    }
	}
	export class Status {
	    currentVersion: string;
	    latestVersion?: string;
	    available: boolean;
	    channel: string;
	    notes?: string;
	    pageUrl?: string;
	    downloadUrl?: string;
	    downloadSha256?: string;
	    downloadSize?: number;
	    publishedAt?: string;
	    checkedAt: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.available = source["available"];
	        this.channel = source["channel"];
	        this.notes = source["notes"];
	        this.pageUrl = source["pageUrl"];
	        this.downloadUrl = source["downloadUrl"];
	        this.downloadSha256 = source["downloadSha256"];
	        this.downloadSize = source["downloadSize"];
	        this.publishedAt = source["publishedAt"];
	        this.checkedAt = source["checkedAt"];
	        this.message = source["message"];
	    }
	}

}

