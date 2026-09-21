import { defineConfig, type Plugin } from 'vite';
import react from '@vitejs/plugin-react';
import fs from 'node:fs';
import path from 'path';

const outDir = path.resolve(__dirname, '../internal/webui/dist');

// emptyOutDir wipes the placeholder that keeps the (otherwise ignored) embed
// directory in git; put it back after every build so `go build` works on a
// fresh checkout and `git status` stays clean.
const keepPlaceholder: Plugin = {
  name: 'go-red-keep-placeholder',
  closeBundle() {
    fs.writeFileSync(path.join(outDir, '.gitkeep'), '');
  },
};

export default defineConfig({
  plugins: [react(), keepPlaceholder],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    // The Go binary embeds this directory (internal/webui/embed.go).
    outDir,
    emptyOutDir: true,
    sourcemap: true,
  },
  server: {
    port: 5173,
    proxy: {
      // The Host header is passed through on purpose: the server's origin
      // policy compares it with the browser's Origin (localhost:5173).
      '/api': {
        target: 'http://localhost:8080',
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
    },
  },
});