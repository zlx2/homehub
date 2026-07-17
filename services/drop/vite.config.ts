import { fileURLToPath, URL } from "node:url";

import vue from "@vitejs/plugin-vue";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [vue()],
  define: {
    "process.env.NODE_ENV": JSON.stringify("production"),
  },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./frontend/src", import.meta.url)),
    },
  },
  build: {
    outDir: fileURLToPath(new URL("./internal/httpapi/web", import.meta.url)),
    emptyOutDir: false,
    sourcemap: false,
    cssCodeSplit: false,
    lib: {
      entry: fileURLToPath(new URL("./frontend/src/main.ts", import.meta.url)),
      formats: ["es"],
      fileName: () => "app.js",
    },
    rollupOptions: {
      output: {
        assetFileNames: (asset) => asset.names?.some((name) => name.endsWith(".css")) ? "app.css" : "[name][extname]",
      },
    },
  },
});
