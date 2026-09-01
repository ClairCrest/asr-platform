import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  // Read .env from the repo root so the whole stack shares one
  // .env.example, same as docker-compose and the Go/Python services.
  envDir: '../',
})
