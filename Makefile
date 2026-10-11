.PHONY: dev dev-web dev-api build test

JWT_SECRET ?= dev

# Run frontend and backend together, then open http://localhost:4321
dev:
	$(MAKE) -j2 dev-web dev-api

dev-web:
	cd web && pnpm run dev

dev-api:
	JWT_SECRET=$(JWT_SECRET) NAWA_DEV=true air

build:
	cd web && pnpm install && pnpm run build
	touch webdist/dist/.gitkeep
	go build -o nawa ./cmd/nawa
