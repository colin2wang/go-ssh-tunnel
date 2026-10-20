export namespace app {
	
	export class SSHInfo {
	    host: string;
	    port: number;
	    user: string;
	    authMethod: string;
	    hasPasswordFile: boolean;
	    hasKeyFile: boolean;
	    hasKeyPassphraseFile: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SSHInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.authMethod = source["authMethod"];
	        this.hasPasswordFile = source["hasPasswordFile"];
	        this.hasKeyFile = source["hasKeyFile"];
	        this.hasKeyPassphraseFile = source["hasKeyPassphraseFile"];
	    }
	}
	export class AppState {
	    version: string;
	    configPath: string;
	    ssh: SSHInfo;
	    tunnels: config.TunnelConfig[];
	
	    static createFrom(source: any = {}) {
	        return new AppState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.configPath = source["configPath"];
	        this.ssh = this.convertValues(source["ssh"], SSHInfo);
	        this.tunnels = this.convertValues(source["tunnels"], config.TunnelConfig);
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
	
	export class TunnelStat {
	    name: string;
	    localPort: number;
	    status: string;
	    error: string;
	    rateUp: number;
	    rateDown: number;
	    totalUp: number;
	    totalDown: number;
	
	    static createFrom(source: any = {}) {
	        return new TunnelStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.localPort = source["localPort"];
	        this.status = source["status"];
	        this.error = source["error"];
	        this.rateUp = source["rateUp"];
	        this.rateDown = source["rateDown"];
	        this.totalUp = source["totalUp"];
	        this.totalDown = source["totalDown"];
	    }
	}
	export class Stats {
	    running: boolean;
	    status: string;
	    upRate: number;
	    downRate: number;
	    totalUp: number;
	    totalDown: number;
	    active: number;
	    tunnels: TunnelStat[];
	
	    static createFrom(source: any = {}) {
	        return new Stats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.status = source["status"];
	        this.upRate = source["upRate"];
	        this.downRate = source["downRate"];
	        this.totalUp = source["totalUp"];
	        this.totalDown = source["totalDown"];
	        this.active = source["active"];
	        this.tunnels = this.convertValues(source["tunnels"], TunnelStat);
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

export namespace config {
	
	export class TunnelConfig {
	    name: string;
	    local_port: number;
	    remote_host: string;
	    remote_port: number;
	
	    static createFrom(source: any = {}) {
	        return new TunnelConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.local_port = source["local_port"];
	        this.remote_host = source["remote_host"];
	        this.remote_port = source["remote_port"];
	    }
	}

}

