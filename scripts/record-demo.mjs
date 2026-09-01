// Records the upload -> transcript flow for the README's animated GIF.
// Not part of the app or its test suite — a manual asset-generation aid.
//
// Setup: npm install playwright && npx playwright install chromium
// Requires the full stack running (docker compose, go run ./cmd/api,
// uv run python -m worker.main, npm run dev) and a short audio file at
// scripts/demo-sample.wav (not committed — generate one, e.g. via
// Windows SAPI or `say`/`espeak`, or point AUDIO_PATH below at any
// short WAV/MP3 you have).
//
// Run:     node scripts/record-demo.mjs
// Convert: ffmpeg -y -i scripts/demo-video/*.webm \
//            -vf "fps=10,scale=800:-1:flags=lanczos,split[s0][s1];[s0]palettegen[p];[s1][p]paletteuse" \
//            docs/images/demo.gif
import { chromium } from 'playwright'

const BASE_URL = process.env.BASE_URL || 'http://localhost:5173'
const AUDIO_PATH = process.env.AUDIO_PATH || './demo-sample.wav'
const VIDEO_DIR = './demo-video'

const email = `demo-${Date.now()}@example.com`
const password = 'hunter2hunter2'

const browser = await chromium.launch()
const context = await browser.newContext({
  viewport: { width: 1200, height: 750 },
  recordVideo: { dir: VIDEO_DIR, size: { width: 1200, height: 750 } },
})
const page = await context.newPage()

await page.goto(`${BASE_URL}/register`)
await page.waitForSelector('text=Create an account')
await page.waitForTimeout(800)

await page.fill('#email', email)
await page.waitForTimeout(300)
await page.fill('#password', password)
await page.waitForTimeout(500)
await page.click('button[type=submit]')
await page.waitForURL('**/jobs')
await page.waitForTimeout(1000)

const fileInput = page.locator('input[type=file]')
await fileInput.setInputFiles(AUDIO_PATH)
await page.waitForSelector('table', { timeout: 15000 })
await page.waitForTimeout(1200)

await page.click('table a')
await page.waitForSelector('text=Attempts')
await page.waitForTimeout(1500)

// Stay on the detail page and let the WebSocket push it to succeeded —
// this is the point of the demo: no reload, no manual polling.
let succeeded = false
for (let i = 0; i < 20; i++) {
  const status = (await page.locator('span.capitalize').first().textContent())?.trim().toLowerCase()
  if (status === 'succeeded') {
    succeeded = true
    break
  }
  await page.waitForTimeout(1000)
}

if (succeeded) {
  await page.waitForSelector('text=Transcript', { timeout: 10000 })
  await page.waitForTimeout(2500)
}

console.log('succeeded:', succeeded)
await context.close()
await browser.close()
console.log('DONE, video saved under', VIDEO_DIR)
