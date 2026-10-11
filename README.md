# Nawa (นาวา)

[![License](https://img.shields.io/github/license/occamist/nawa)](https://github.com/occamist/laverna/blob/main/nawa)

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

If you have the binary, below will run on localhost:5555. On first run, if no admin password is set, one is generated and printed to the terminal only ONCE!

```sh
JWT_SECRET=your-secret nawa
```

## Development

You can start the dev servers together on localhost:4321

```sh
make dev
```
