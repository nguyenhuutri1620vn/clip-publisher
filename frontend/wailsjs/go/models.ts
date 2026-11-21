export namespace config {
	
	export class Settings {
	    video_repo: string;
	    voice_repo: string;
	    video_python: string;
	    voice_bin: string;
	    claude_bin: string;
	    workspace: string;
	    default_voice: string;
	    default_music: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.video_repo = source["video_repo"];
	        this.voice_repo = source["voice_repo"];
	        this.video_python = source["video_python"];
	        this.voice_bin = source["voice_bin"];
	        this.claude_bin = source["claude_bin"];
	        this.workspace = source["workspace"];
	        this.default_voice = source["default_voice"];
	        this.default_music = source["default_music"];
	    }
	}

}

export namespace pipeline {
	
	export class Check {
	    name: string;
	    ok: boolean;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new Check(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ok = source["ok"];
	        this.detail = source["detail"];
	    }
	}

}

export namespace scriptmodel {
	
	export class SlideChange {
	    id: string;
	    kind: string;
	    fields?: string[];
	
	    static createFrom(source: any = {}) {
	        return new SlideChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.fields = source["fields"];
	    }
	}
	export class Warning {
	    slideId: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Warning(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slideId = source["slideId"];
	        this.message = source["message"];
	    }
	}

}

export namespace service {
	
	export class ProgressDTO {
	    step: string;
	    voiceDone: number;
	    voiceId: string;
	    videoId: number;
	    kind: string;
	    voiceTotal: number;
	    queuePos: number;
	    elapsedSec: number;
	    etaSec: number;
	
	    static createFrom(source: any = {}) {
	        return new ProgressDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.step = source["step"];
	        this.voiceDone = source["voiceDone"];
	        this.voiceId = source["voiceId"];
	        this.videoId = source["videoId"];
	        this.kind = source["kind"];
	        this.voiceTotal = source["voiceTotal"];
	        this.queuePos = source["queuePos"];
	        this.elapsedSec = source["elapsedSec"];
	        this.etaSec = source["etaSec"];
	    }
	}
	export class RenderDTO {
	    id: number;
	    version: number;
	    kind: string;
	    state: string;
	    url: string;
	    error: string;
	    seconds: number;
	    startedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new RenderDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.version = source["version"];
	        this.kind = source["kind"];
	        this.state = source["state"];
	        this.url = source["url"];
	        this.error = source["error"];
	        this.seconds = source["seconds"];
	        this.startedAt = source["startedAt"];
	    }
	}
	export class SlideDTO {
	    id: string;
	    title: string;
	    subtitle: string;
	    narration: string;
	    sceneSummary: string;
	    note: string;
	    timeHint: string;
	    claim: boolean;
	    needsCheck: boolean;
	    estSeconds: number;
	    previewUrl: string;
	    startSec: number;
	    rawJson: string;
	
	    static createFrom(source: any = {}) {
	        return new SlideDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.subtitle = source["subtitle"];
	        this.narration = source["narration"];
	        this.sceneSummary = source["sceneSummary"];
	        this.note = source["note"];
	        this.timeHint = source["timeHint"];
	        this.claim = source["claim"];
	        this.needsCheck = source["needsCheck"];
	        this.estSeconds = source["estSeconds"];
	        this.previewUrl = source["previewUrl"];
	        this.startSec = source["startSec"];
	        this.rawJson = source["rawJson"];
	    }
	}
	export class SourceDTO {
	    text: string;
	    needsCheck: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SourceDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.needsCheck = source["needsCheck"];
	    }
	}
	export class VersionDTO {
	    n: number;
	    author: string;
	    summary: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new VersionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.n = source["n"];
	        this.author = source["author"];
	        this.summary = source["summary"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class VideoSummary {
	    id: number;
	    title: string;
	    status: string;
	    statusDetail: string;
	    origin: string;
	    editable: boolean;
	    duration: number;
	    updatedAt: string;
	    thumbUrl: string;
	    queuePos: number;
	    location: string;
	
	    static createFrom(source: any = {}) {
	        return new VideoSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.statusDetail = source["statusDetail"];
	        this.origin = source["origin"];
	        this.editable = source["editable"];
	        this.duration = source["duration"];
	        this.updatedAt = source["updatedAt"];
	        this.thumbUrl = source["thumbUrl"];
	        this.queuePos = source["queuePos"];
	        this.location = source["location"];
	    }
	}
	export class VideoDetail {
	    video: VideoSummary;
	    voice: string;
	    music: string;
	    currentVersion: number;
	    versions: VersionDTO[];
	    slides: SlideDTO[];
	    sources: SourceDTO[];
	    warnings: scriptmodel.Warning[];
	    changes: scriptmodel.SlideChange[];
	    renders: RenderDTO[];
	    chat: workspace.ChatMessage[];
	    logTail: string[];
	    progress?: ProgressDTO;
	
	    static createFrom(source: any = {}) {
	        return new VideoDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.video = this.convertValues(source["video"], VideoSummary);
	        this.voice = source["voice"];
	        this.music = source["music"];
	        this.currentVersion = source["currentVersion"];
	        this.versions = this.convertValues(source["versions"], VersionDTO);
	        this.slides = this.convertValues(source["slides"], SlideDTO);
	        this.sources = this.convertValues(source["sources"], SourceDTO);
	        this.warnings = this.convertValues(source["warnings"], scriptmodel.Warning);
	        this.changes = this.convertValues(source["changes"], scriptmodel.SlideChange);
	        this.renders = this.convertValues(source["renders"], RenderDTO);
	        this.chat = this.convertValues(source["chat"], workspace.ChatMessage);
	        this.logTail = source["logTail"];
	        this.progress = this.convertValues(source["progress"], ProgressDTO);
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

export namespace workspace {
	
	export class ChatMessage {
	    role: string;
	    text: string;
	    // Go type: time
	    at: any;
	
	    static createFrom(source: any = {}) {
	        return new ChatMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.text = source["text"];
	        this.at = this.convertValues(source["at"], null);
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

