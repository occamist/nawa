# Known Issues

## Backend

### linter issues

### main.go
- os.Exit(1) kills defers too quickly, fix them


## Frontend

### `web/src/stores/auth.ts`

- `logout()` swallows fetch errors — user gets stuck
- `isAuthenticated` localStorage flag is a UX gate, not documented as such

### Pages

- `dashboard.astro`: No 401 check on images fetch; container links all go to `/containers`
- `containers/logs.astro`: Execution continues after redirect when `!id`; no SSE reconnection; unbounded DOM growth
- `images/index.astro`: `tag.split(':')` breaks on registry ports; 401 race condition

### Fetchers

- must use Zod and make try/catch blocks safer 
```ts
import { z } from "zod"; // Optional: For actual runtime validation

// Define a schema if you want to be 100% safe
const ImageArraySchema = z.array(z.object({
    id: z.string(),
    url: z.string(),
    // ... other image properties
}));

async function fetchImages(): Promise<Image[]> {
    try {
        const res = await fetch(RouteV1.Images, { credentials: "include" });

        if (res.status === 401) {
            clearAuth();
            window.location.href = "/";
            // Return a pending promise that never resolves to halt further execution 
            // while the page redirects
            return new Promise(() => {}); 
        }

        if (!res.ok) {
            const errorText = await res.text();
            throw new Error(`HTTP ${res.status}: ${errorText}`);
        }

        const data = await res.json();
        
        // Option A: Strict runtime validation (Recommended)
        // return ImageArraySchema.parse(data); 

        // Option B: Explicit type assertion (At least tells TS you are forcing the type)
        return data as Image[]; 

    } catch (error) {
        console.error("Failed to fetch images:", error);
        // Re-throw or return a safe fallback depending on your app's error strategy
        throw error; 
    }
}
```
