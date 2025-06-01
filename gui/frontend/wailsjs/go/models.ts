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
	    }
	}
	export class UploadResult {
	    success: boolean;
	    snippet: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new UploadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.snippet = source["snippet"];
	        this.error = source["error"];
	    }
	}

}

