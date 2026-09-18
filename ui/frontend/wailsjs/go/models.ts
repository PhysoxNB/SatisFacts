export namespace editor {
	
	export class BeltInventoryEntry {
	    type: string;
	    mk: number;
	    count: number;
	    className?: string;
	
	    static createFrom(source: any = {}) {
	        return new BeltInventoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.mk = source["mk"];
	        this.count = source["count"];
	        this.className = source["className"];
	    }
	}
	export class BlueprintSubCategory {
	    subCategoryName: string;
	    menuPriority: number;
	    isUndefined: boolean;
	    blueprintNames: string[];
	
	    static createFrom(source: any = {}) {
	        return new BlueprintSubCategory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.subCategoryName = source["subCategoryName"];
	        this.menuPriority = source["menuPriority"];
	        this.isUndefined = source["isUndefined"];
	        this.blueprintNames = source["blueprintNames"];
	    }
	}
	export class BlueprintCategoryRecord {
	    categoryName: string;
	    iconId: number;
	    menuPriority: number;
	    isUndefined: boolean;
	    subCategories: BlueprintSubCategory[];
	
	    static createFrom(source: any = {}) {
	        return new BlueprintCategoryRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.categoryName = source["categoryName"];
	        this.iconId = source["iconId"];
	        this.menuPriority = source["menuPriority"];
	        this.isUndefined = source["isUndefined"];
	        this.subCategories = this.convertValues(source["subCategories"], BlueprintSubCategory);
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
	
	export class FixSessionNameResult {
	    outputPath: string;
	    originalName: string;
	    newName: string;
	    bodyReplacements: number;
	
	    static createFrom(source: any = {}) {
	        return new FixSessionNameResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.outputPath = source["outputPath"];
	        this.originalName = source["originalName"];
	        this.newName = source["newName"];
	        this.bodyReplacements = source["bodyReplacements"];
	    }
	}
	export class SessionNameIssue {
	    type: string;
	    message: string;
	    chars?: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionNameIssue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.message = source["message"];
	        this.chars = source["chars"];
	    }
	}
	export class SessionNameCheckResult {
	    hasIssues: boolean;
	    issues: SessionNameIssue[];
	    autoFix: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionNameCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasIssues = source["hasIssues"];
	        this.issues = this.convertValues(source["issues"], SessionNameIssue);
	        this.autoFix = source["autoFix"];
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

export namespace main {
	
	export class AnalyzeResult {
	    htmlPath: string;
	    elapsed: string;
	    totalObjects: number;
	    totalBuildings: number;
	    htmlSizeKB: number;
	    canOptimize: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AnalyzeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.htmlPath = source["htmlPath"];
	        this.elapsed = source["elapsed"];
	        this.totalObjects = source["totalObjects"];
	        this.totalBuildings = source["totalBuildings"];
	        this.htmlSizeKB = source["htmlSizeKB"];
	        this.canOptimize = source["canOptimize"];
	    }
	}
	export class BeltInventoryInfo {
	    items: editor.BeltInventoryEntry[];
	    hubTier: number;
	    maxBeltMk: number;
	    maxLiftMk: number;
	    maxPipeMk: number;
	
	    static createFrom(source: any = {}) {
	        return new BeltInventoryInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], editor.BeltInventoryEntry);
	        this.hubTier = source["hubTier"];
	        this.maxBeltMk = source["maxBeltMk"];
	        this.maxLiftMk = source["maxLiftMk"];
	        this.maxPipeMk = source["maxPipeMk"];
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
	export class BeltUpgradeOptionsFrontend {
	    upgradeBelts: boolean;
	    upgradeLifts: boolean;
	    upgradePipes: boolean;
	    targetMk: number;
	    targetPipeMk: number;
	
	    static createFrom(source: any = {}) {
	        return new BeltUpgradeOptionsFrontend(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.upgradeBelts = source["upgradeBelts"];
	        this.upgradeLifts = source["upgradeLifts"];
	        this.upgradePipes = source["upgradePipes"];
	        this.targetMk = source["targetMk"];
	        this.targetPipeMk = source["targetPipeMk"];
	    }
	}
	export class ConfigFileInfo {
	    exists: boolean;
	    path: string;
	    hasEntry: boolean;
	    hasBackup: boolean;
	    readOnly: boolean;
	    content: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigFileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exists = source["exists"];
	        this.path = source["path"];
	        this.hasEntry = source["hasEntry"];
	        this.hasBackup = source["hasBackup"];
	        this.readOnly = source["readOnly"];
	        this.content = source["content"];
	        this.error = source["error"];
	    }
	}
	export class CopyBlueprintCategoriesResult {
	    sourceCategories: number;
	    targetCategories: number;
	    finalCategories: number;
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new CopyBlueprintCategoriesResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceCategories = source["sourceCategories"];
	        this.targetCategories = source["targetCategories"];
	        this.finalCategories = source["finalCategories"];
	        this.mode = source["mode"];
	    }
	}
	export class DepotItemInfo {
	    index: number;
	    itemClass: string;
	    displayName: string;
	    amount: number;
	    stackSize: number;
	
	    static createFrom(source: any = {}) {
	        return new DepotItemInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.itemClass = source["itemClass"];
	        this.displayName = source["displayName"];
	        this.amount = source["amount"];
	        this.stackSize = source["stackSize"];
	    }
	}
	export class ExportBlueprintPresetResult {
	    categories: number;
	
	    static createFrom(source: any = {}) {
	        return new ExportBlueprintPresetResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.categories = source["categories"];
	    }
	}
	export class GameConfigStatus {
	    configDir: string;
	    engineIni: ConfigFileInfo;
	    inputIni: ConfigFileInfo;
	
	    static createFrom(source: any = {}) {
	        return new GameConfigStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configDir = source["configDir"];
	        this.engineIni = this.convertValues(source["engineIni"], ConfigFileInfo);
	        this.inputIni = this.convertValues(source["inputIni"], ConfigFileInfo);
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
	export class MemoryCheck {
	    neededGB: number;
	    availableGB: number;
	    enough: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MemoryCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.neededGB = source["neededGB"];
	        this.availableGB = source["availableGB"];
	        this.enough = source["enough"];
	    }
	}
	export class MultiplierInfo {
	    name: string;
	    label: string;
	    current: number;
	    target: number;
	    uiMax: number;
	
	    static createFrom(source: any = {}) {
	        return new MultiplierInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.label = source["label"];
	        this.current = source["current"];
	        this.target = source["target"];
	        this.uiMax = source["uiMax"];
	    }
	}
	export class OptimizeResultInfo {
	    originalSizeMB: number;
	    newSizeMB: number;
	    savingsMB: number;
	    savingsPct: number;
	    totalObjects: number;
	    levelCount: number;
	
	    static createFrom(source: any = {}) {
	        return new OptimizeResultInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.originalSizeMB = source["originalSizeMB"];
	        this.newSizeMB = source["newSizeMB"];
	        this.savingsMB = source["savingsMB"];
	        this.savingsPct = source["savingsPct"];
	        this.totalObjects = source["totalObjects"];
	        this.levelCount = source["levelCount"];
	    }
	}
	export class RecentSave {
	    path: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new RecentSave(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	    }
	}
	export class SaveFileInfo {
	    name: string;
	    path: string;
	    size: number;
	    modTime: string;
	    selected: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SaveFileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.modTime = source["modTime"];
	        this.selected = source["selected"];
	    }
	}
	export class SaveSummary {
	    fileName: string;
	    filePath: string;
	    fileSize: number;
	    decompressedMB: number;
	    sessionName: string;
	    saveVersion: number;
	    isV2: boolean;
	    maxChunkSize: number;
	
	    static createFrom(source: any = {}) {
	        return new SaveSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileName = source["fileName"];
	        this.filePath = source["filePath"];
	        this.fileSize = source["fileSize"];
	        this.decompressedMB = source["decompressedMB"];
	        this.sessionName = source["sessionName"];
	        this.saveVersion = source["saveVersion"];
	        this.isV2 = source["isV2"];
	        this.maxChunkSize = source["maxChunkSize"];
	    }
	}
	export class Settings {
	    recentSaves: RecentSave[];
	    defaultOutputLoc: string;
	    customOutputDir: string;
	    autoDetectSave: boolean;
	    autoAnalyze: boolean;
	    customSaveDir: string;
	    language: string;
	    zoomFactor: number;
	    windowWidth: number;
	    windowHeight: number;
	    mapColors?: Record<string, string>;
	    includeMap?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.recentSaves = this.convertValues(source["recentSaves"], RecentSave);
	        this.defaultOutputLoc = source["defaultOutputLoc"];
	        this.customOutputDir = source["customOutputDir"];
	        this.autoDetectSave = source["autoDetectSave"];
	        this.autoAnalyze = source["autoAnalyze"];
	        this.customSaveDir = source["customSaveDir"];
	        this.language = source["language"];
	        this.zoomFactor = source["zoomFactor"];
	        this.windowWidth = source["windowWidth"];
	        this.windowHeight = source["windowHeight"];
	        this.mapColors = source["mapColors"];
	        this.includeMap = source["includeMap"];
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
	export class SettingsPatch {
	    defaultOutputLoc?: string;
	    customOutputDir?: string;
	    autoDetectSave?: boolean;
	    autoAnalyze?: boolean;
	    customSaveDir?: string;
	    language?: string;
	    zoomFactor?: number;
	    windowWidth?: number;
	    windowHeight?: number;
	    mapColors?: Record<string, string>;
	    includeMap?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SettingsPatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultOutputLoc = source["defaultOutputLoc"];
	        this.customOutputDir = source["customOutputDir"];
	        this.autoDetectSave = source["autoDetectSave"];
	        this.autoAnalyze = source["autoAnalyze"];
	        this.customSaveDir = source["customSaveDir"];
	        this.language = source["language"];
	        this.zoomFactor = source["zoomFactor"];
	        this.windowWidth = source["windowWidth"];
	        this.windowHeight = source["windowHeight"];
	        this.mapColors = source["mapColors"];
	        this.includeMap = source["includeMap"];
	    }
	}
	export class UpdateCheckResult {
	    currentVersion: string;
	    latestVersion: string;
	    updateAvailable: boolean;
	    releaseURL: string;
	    releaseNotes: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.updateAvailable = source["updateAvailable"];
	        this.releaseURL = source["releaseURL"];
	        this.releaseNotes = source["releaseNotes"];
	    }
	}

}

