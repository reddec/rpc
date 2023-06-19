// Calc is API server.
// Multiple line
// docs are
// also supported
export default class Calc {

    constructor(private readonly baseURL: string = ".") {}
    
    // Name of the person
    async Name(prefix: string): Promise<calc> {
        return (await this.invoke("name", [ prefix ])) as calc
    }
    
    async Today(): Promise<string> {
        return (await this.invoke("today", [  ])) as string
    }
    
    // Update something
    async Update(tp: SomeType): Promise<void> {
        await this.invoke("update", [ tp ])
    }
    
    async Binary(): Promise<string> {
        return (await this.invoke("binary", [  ])) as string
    }
    
    async Bool(): Promise<boolean> {
        return (await this.invoke("bool", [  ])) as boolean
    }
    
    async Custom(): Promise<Encoder> {
        return (await this.invoke("custom", [  ])) as Encoder
    }
    
    async Nillable(): Promise<(string | null)> {
        return (await this.invoke("nillable", [  ])) as (string | null)
    }
    
    async Error(): Promise<void> {
        await this.invoke("error", [  ])
    }
    
    async AnotherTime(): Promise<Encoder1> {
        return (await this.invoke("anothertime", [  ])) as Encoder1
    }
    
    async NillableSlice(enc: Encoder1): Promise<((string | null)[] | null)> {
        return (await this.invoke("nillableslice", [ enc ])) as ((string | null)[] | null)
    }
    
    async AnonType(): Promise<Anon> {
        return (await this.invoke("anontype", [  ])) as Anon
    }
    
    async Multiple(name: string, a: number, b: number, ts: string): Promise<boolean> {
        return (await this.invoke("multiple", [ name, a, b, ts ])) as boolean
    }
    
    private async invoke(method: string, args: any[]): Promise<any> {
        const res = await fetch(this.baseURL + "/" + encodeURIComponent(method), {
            method: "POST",
            body: JSON.stringify(args),
            headers: {
                "Content-Type": "application/json"
            }
        })
        if (!res.ok) throw new Error(await res.text());
        return await res.json()
    }
}

export interface Anon {
    X: number
}

export interface Encoder {
}

export interface SomeType {
    Name: string
    the_age: number
    Enabled?: boolean
    Ref: (number | null)
    Location: string
    Document: any
}

export interface calc {
}

export type Encoder1 = number;
