export default function RPC(baseURL = "") {
    return new Proxy({}, {
        get(obj, method) {
            method = method.toLowerCase();
            if (method in obj) return obj[method]
            return obj[method] = async () => {
                const res = await fetch(baseURL + "/" + encodeURIComponent(method), {
                    method: "POST",
                    body: JSON.stringify(Array.prototype.slice.call(arguments)),
                    headers: {
                        "Content-Type": "application/json"
                    }
                })
                if (!res.ok) throw new Error(await res.text());
                return await res.json()
            }
        }
    })
}