export namespace main {

	export class OpenResult {
	    path: string;
	    content: string;
	    cancelled: boolean;

	    static createFrom(source: any = {}) {
	        return new OpenResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.content = source["content"];
	        this.cancelled = source["cancelled"];
	    }
	}
	export class SaveResult {
	    path: string;
	    cancelled: boolean;

	    static createFrom(source: any = {}) {
	        return new SaveResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.cancelled = source["cancelled"];
	    }
	}

}

export namespace tectonic {

	export class Result {
	    pdf: number[];
	    log: string;
	    ok: boolean;

	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pdf = source["pdf"];
	        this.log = source["log"];
	        this.ok = source["ok"];
	    }
	}

}
