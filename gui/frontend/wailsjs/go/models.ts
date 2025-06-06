export namespace main {
	
	export class MultiPhotoImageData {
	    path: string;
	    alt: string;
	    title: string;
	    description: string;
	    isFromPhotos: boolean;
	    photosIndex: number;
	    photosId: string;
	
	    static createFrom(source: any = {}) {
	        return new MultiPhotoImageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.alt = source["alt"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.isFromPhotos = source["isFromPhotos"];
	        this.photosIndex = source["photosIndex"];
	        this.photosId = source["photosId"];
	    }
	}
	export class MultiPhotoOutputResult {
	    path: string;
	    url: string;
	    alt: string;
	    markdown?: string;
	    html?: string;
	    error?: string;
	    duplicate: boolean;
	    warnings?: string[];
	
	    static createFrom(source: any = {}) {
	        return new MultiPhotoOutputResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.url = source["url"];
	        this.alt = source["alt"];
	        this.markdown = source["markdown"];
	        this.html = source["html"];
	        this.error = source["error"];
	        this.duplicate = source["duplicate"];
	        this.warnings = source["warnings"];
	    }
	}
	export class MultiPhotoUploadRequest {
	    post: string;
	    images: MultiPhotoImageData[];
	    tags: string[];
	    mastodon: boolean;
	    bluesky: boolean;
	    visibility: string;
	    format: string;
	
	    static createFrom(source: any = {}) {
	        return new MultiPhotoUploadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.post = source["post"];
	        this.images = this.convertValues(source["images"], MultiPhotoImageData);
	        this.tags = source["tags"];
	        this.mastodon = source["mastodon"];
	        this.bluesky = source["bluesky"];
	        this.visibility = source["visibility"];
	        this.format = source["format"];
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
	export class MultiPhotoUploadResult {
	    success: boolean;
	    outputs: MultiPhotoOutputResult[];
	    error?: string;
	    socialStatus?: string;
	    isPullMode?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MultiPhotoUploadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.outputs = this.convertValues(source["outputs"], MultiPhotoOutputResult);
	        this.error = source["error"];
	        this.socialStatus = source["socialStatus"];
	        this.isPullMode = source["isPullMode"];
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
	export class PhotoMetadata {
	    path: string;
	    title: string;
	    alt: string;
	    description: string;
	    tags: string[];
	    format: string;
	    private: boolean;
	    mastodonEnabled: boolean;
	    mastodonText: string;
	    mastodonVisibility: string;
	    blueskyEnabled: boolean;
	    blueskyText: string;
	    thumbnail: string;
	    imageWidth: number;
	    imageHeight: number;
	    fileSize: number;
	    isTemporary: boolean;
	    isFromPhotos: boolean;
	    photosIndex: number;
	    photosId: string;
	    photosFilename: string;
	
	    static createFrom(source: any = {}) {
	        return new PhotoMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.title = source["title"];
	        this.alt = source["alt"];
	        this.description = source["description"];
	        this.tags = source["tags"];
	        this.format = source["format"];
	        this.private = source["private"];
	        this.mastodonEnabled = source["mastodonEnabled"];
	        this.mastodonText = source["mastodonText"];
	        this.mastodonVisibility = source["mastodonVisibility"];
	        this.blueskyEnabled = source["blueskyEnabled"];
	        this.blueskyText = source["blueskyText"];
	        this.thumbnail = source["thumbnail"];
	        this.imageWidth = source["imageWidth"];
	        this.imageHeight = source["imageHeight"];
	        this.fileSize = source["fileSize"];
	        this.isTemporary = source["isTemporary"];
	        this.isFromPhotos = source["isFromPhotos"];
	        this.photosIndex = source["photosIndex"];
	        this.photosId = source["photosId"];
	        this.photosFilename = source["photosFilename"];
	    }
	}
	export class UploadResult {
	    success: boolean;
	    snippet: string;
	    error?: string;
	    duplicate: boolean;
	    forceAvailable: boolean;
	    socialPostStatus?: string;
	
	    static createFrom(source: any = {}) {
	        return new UploadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.snippet = source["snippet"];
	        this.error = source["error"];
	        this.duplicate = source["duplicate"];
	        this.forceAvailable = source["forceAvailable"];
	        this.socialPostStatus = source["socialPostStatus"];
	    }
	}

}

export namespace types {
	
	export class ImageSizes {
	    large: string;
	    medium: string;
	    small: string;
	    thumb: string;
	
	    static createFrom(source: any = {}) {
	        return new ImageSizes(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.large = source["large"];
	        this.medium = source["medium"];
	        this.small = source["small"];
	        this.thumb = source["thumb"];
	    }
	}
	export class PullImage {
	    id: string;
	    title: string;
	    description?: string;
	    source_url: string;
	    sizes: ImageSizes;
	    alt: string;
	    tags?: string[];
	
	    static createFrom(source: any = {}) {
	        return new PullImage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.source_url = source["source_url"];
	        this.sizes = this.convertValues(source["sizes"], ImageSizes);
	        this.alt = source["alt"];
	        this.tags = source["tags"];
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
	export class PullSource {
	    service: string;
	    album?: string;
	
	    static createFrom(source: any = {}) {
	        return new PullSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.service = source["service"];
	        this.album = source["album"];
	    }
	}
	export class PullRequest {
	    source: PullSource;
	    post: string;
	    images: PullImage[];
	    targets?: string[];
	    visibility?: string;
	    format?: string;
	
	    static createFrom(source: any = {}) {
	        return new PullRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = this.convertValues(source["source"], PullSource);
	        this.post = source["post"];
	        this.images = this.convertValues(source["images"], PullImage);
	        this.targets = source["targets"];
	        this.visibility = source["visibility"];
	        this.format = source["format"];
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

