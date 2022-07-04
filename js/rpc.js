export default function RPC(baseURL = "") {
    return new Proxy({}, {
        get(obj, method) {
            method = method.toLowerCase();
            if (method in obj) {
                return obj[method]
            }

            const url = baseURL + "/" + encodeURIComponent(method)
            const fn = async function () {
                const args = Array.prototype.slice.call(arguments);
                const res = await fetch(url, {
                    method: "POST",
                    body: JSON.stringify(args),
                    headers: {
                        "Content-Type": "application/json"
                    }
                })
                if (!res.ok) {
                    const errMessage = await res.text();
                    throw new Error(errMessage);
                }
                return await res.json()
            }
            return obj[method] = fn
        }
    })
}