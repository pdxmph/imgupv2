export namespace main {
	
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

