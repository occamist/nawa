# Nawa (นาวา)

A self-hosted container management for single host machines, served as a single binary. Nawa means a ship or a vessel in Thai. 

I have built it solely for personal uses. Inspired by the designs of dockge (minimalist) & arcane (pragmatic). I just wanted the best of both worlds for myself.

## Building the Binary

```sh
cd web
pnpm install && pnpm run build
cd ..
go build -o nawa ./cmd/nawa
```

## Configuration

All configuration is via environment variables.

| Variable         | Required | Default   | Description                        |
|------------------|----------|-----------|------------------------------------|
| `JWT_SECRET`     | yes      |           | Secret used to sign JWT tokens     |
| `ADMIN_USERNAME` | no       | `admin`   | Admin username                     |
| `ADMIN_PASSWORD` | no       | generated if unset | Admin password            |
| `COOKIE_SECURE`  | no       | `false`   | Set to `true` behind HTTPS         |

## Getting Started

If you have the binary built, it has both frontend & backend in it.

```sh
JWT_SECRET=your-secret nawa
```

Then open `http://localhost:5555`.

On first run, if no admin password is set, one is generated and printed to the terminal, please save it, it is shown only once.

## Development

Run the backend and frontend dev server separately for live file changes:

```sh
# Frontend @ localhost:4321
cd web && pnpm run dev

# Backend @ localhost:5555
JWT_SECRET=dev air
```
