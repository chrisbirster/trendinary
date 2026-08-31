import { defineConfig } from "vite";
import solid from "@solidjs/vite-plugin";
import stylex from "@stylexjs/unplugin";

export default defineConfig({
  plugins: [
    stylex.vite({
      unstable_moduleResolution: {
        type: "commonJS",
        rootDir: process.cwd(),
      },
    }),
    solid(),
  ],
  build: {
    outDir: "internal/web/dist",
    emptyOutDir: false,
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      "/api": "http://127.0.0.1:8080",
    },
  },
});
