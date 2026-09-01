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

