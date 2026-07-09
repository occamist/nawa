# Known Issues

## Backend

### linter issues

### main.go
- os.Exit(1) kills defers too quickly, fix them

### content types across encoders need set application/json

- w.Header().Set("Content-Type", "application/json")

## Frontend

### `web/src/stores/auth.ts`

- `logout()` swallows fetch errors — user gets stuck
- `isAuthenticated` localStorage flag is a UX gate, not documented as such

### Pages

- `dashboard.astro`: No 401 check on images fetch; container links all go to `/containers`
- `containers/logs.astro`: Execution continues after redirect when `!id`; no SSE reconnection; unbounded DOM growth
- `images/index.astro`: `tag.split(':')` breaks on registry ports; 401 race condition


