export namespace backend {
	
	export class Peer {
	    id: string;
	    hostname: string;
	    ip: string;
	    port: number;
	    isSelf: boolean;
	    lastSeen: number;
	
	    static createFrom(source: any = {}) {
	        return new Peer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.hostname = source["hostname"];
	        this.ip = source["ip"];
	        this.port = source["port"];
	        this.isSelf = source["isSelf"];
	        this.lastSeen = source["lastSeen"];
	    }
	}

}

