import react from "@vitejs/plugin-react";
import {defineConfig} from "vitest/config";

export default defineConfig({
    plugins: [react()],
    build: {
        outDir: "../backend/internal/webui/dist",
        emptyOutDir: true,
    },
    server: {
        host: true,
        port: 3000,
        strictPort: true,
        allowedHosts: ["localhost"],
        watch: {
            usePolling: true,
            interval: 100,
        },
    },
});
