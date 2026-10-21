import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": "http://localhost:8081",
      "/health": "http://localhost:8081",
      "/openapi.json": "http://localhost:8081",
      "/swagger": "http://localhost:8081"
    }
  }
});

