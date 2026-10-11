# Nawa (นาวา)

[![License](https://img.shields.io/github/license/occamist/laverna)](https://github.com/occamist/laverna/blob/main/LICENSE)

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

Start the frontend dev server and the backend together, then open `http://localhost:4321`:

```sh
make dev
```
