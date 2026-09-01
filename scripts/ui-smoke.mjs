// Manual UI verification aid — not part of the app or its test suite.
// Drives the dashboard end to end (register, upload, wait for
// transcription, view transcript, create an API key) and screenshots
// each step. Requires the full stack running (docker compose, go run
// ./cmd/api, uv run python -m worker.main, npm run dev) and an audio
// sample at scripts/sample-speech.wav (not committed — generate one, e.g.
// via Windows SAPI or `say`/`espeak`, or point AUDIO_PATH below at any
// short WAV/MP3 you have).
//
// Setup: npm install playwright && npx playwright install chromium
// Run:   node scripts/ui-smoke.mjs
import { chromium } from 'playwright'

const email = `ui-${Date.now()}@example.com`
const password = 'hunter2hunter2'

const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 1280, height: 800 } })
const shots = []

const shot = async (name) => {
  const path = `D:/Work/self-project/EnglishASRPlatform/scripts/screenshots/${name}.png`
  await page.screenshot({ path })
  shots.push(path)
  console.log('screenshot:', path)
}

page.on('console', (msg) => {
  if (msg.type() === 'error') console.log('CONSOLE ERROR:', msg.text())
})
page.on('pageerror', (err) => console.log('PAGE ERROR:', err.message))

await page.goto('http://localhost:5173/register')
await page.waitForSelector('text=Create an account')
await shot('01-register')

await page.fill('#email', email)
await page.fill('#password', password)
await page.click('button[type=submit]')
await page.waitForURL('**/jobs')
await shot('02-jobs-empty')

const fileInput = page.locator('input[type=file]')
await fileInput.setInputFiles('D:/Work/self-project/EnglishASRPlatform/scripts/sample-speech.wav')
await page.waitForSelector('text=/Uploading|Creating job/', { timeout: 5000 }).catch(() => {})
await shot('03-uploading')

await page.waitForSelector('table', { timeout: 15000 })
await shot('04-jobs-list')

await page.click('table a')
await page.waitForSelector('text=Attempts')
await shot('05-job-detail')

// Poll for completion (up to ~60s) by reloading the detail page.
let succeeded = false
for (let i = 0; i < 20; i++) {
  const badge = await page.locator('span.capitalize').first().textContent()
  if (badge?.trim().toLowerCase() === 'succeeded') {
    succeeded = true
    break
  }
  await page.waitForTimeout(3000)
  await page.reload()
}
await shot('06-job-final')

if (succeeded) {
  await page.waitForSelector('text=Transcript', { timeout: 10000 })
  await shot('07-transcript')
}

await page.goto('http://localhost:5173/api-keys')
await page.waitForSelector('text=API Keys')
await page.fill('input[type=text]', 'smoke-test-key')
await page.click('button:has-text("Create key")')
await page.waitForSelector('text=Copy this key now')
await shot('08-api-key-created')

console.log('SUCCEEDED:', succeeded)
console.log('DONE')
await browser.close()
