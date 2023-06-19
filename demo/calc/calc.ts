// Calc is API server.
// Multiple line
// docs are
// also supported
export default class Calc {

    constructor(private readonly baseURL: string = ".") {}
    
    // Name of the person
    async Name(prefix: string): Promise<calc> {
        return (await this.invoke("Name", [ prefix ])) as calc
    }
    
    async Today(): Promise<string> {
        return (await this.invoke("Today", [  ])) as string
    }
    
    // Update something
    async Update(tp: SomeType): Promise<void> {
        await this.invoke("Update", [ tp ])
    }
    
    async Binary(): Promise<string> {
        return (await this.invoke("Binary", [  ])) as string
    }
    
    async Bool(): Promise<boolean> {
        return (await this.invoke("Bool", [  ])) as boolean
    }
    
    async Custom(): Promise<Encoder> {
        return (await this.invoke("Custom", [  ])) as Encoder
    }
    
    async Nillable(): Promise<(string | null)> {
        return (await this.invoke("Nillable", [  ])) as (string | null)
    }
    
    async Error(): Promise<void> {
        await this.invoke("Error", [  ])
    }
    
    async AnotherTime(): Promise<Encoder1> {
        return (await this.invoke("AnotherTime", [  ])) as Encoder1
    }
    
    async NillableSlice(enc: Encoder1): Promise<((string | null)[] | null)> {
        return (await this.invoke("NillableSlice", [ enc ])) as ((string | null)[] | null)
    }
    
    async AnonType(): Promise<Anon> {
        return (await this.invoke("AnonType", [  ])) as Anon
    }
    
    async Multiple(name: string, a: number, b: number, ts: string): Promise<boolean> {
        return (await this.invoke("Multiple", [ name, a, b, ts ])) as boolean
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
}

export interface calc {
}

export type Encoder1 = number;
