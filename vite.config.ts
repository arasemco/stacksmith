import { defineConfig } from "vite";

export default defineConfig({
  base: "./",
  server: {
    proxy: {
      "/v1": "http://localhost:8080",
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
