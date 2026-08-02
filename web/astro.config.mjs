// @ts-check
import { defineConfig } from "astro/config";
import scope from "astro-scope";

export default defineConfig({
  integrations: [scope()],
  outDir: "../webdist/dist",
  server: {
    port: 4321,
  },
  vite: {
    server: {
      proxy: {
        "/api": "http://localhost:5555",
        "/healthz": "http://localhost:5555",
      },
    },
  },
});
