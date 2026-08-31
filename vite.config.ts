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
  server: {
    port: 5173,
    strictPort: true,
  },
});
