// Verifies the WebSocket live-update path specifically: navigate to a job
// detail page and NEVER reload it, just wait — the status badge should
// flip from processing to succeeded on its own via useJobStream's query
// invalidation. This is distinct from ui-smoke.mjs, which polls via
// page.reload() and therefore doesn't prove the no-refresh requirement.
import { chromium } from 'playwright'

const email = `live-${Date.now()}@example.com`
const password = 'hunter2hunter2'

const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 1000, height: 700 } })
page.on('console', (msg) => {
  if (msg.type() === 'error') console.log('CONSOLE ERROR:', msg.text())
})

await page.goto('http://localhost:5173/register')
await page.fill('#email', email)
await page.fill('#password', password)
await page.click('button[type=submit]')
await page.waitForURL('**/jobs')

await page.locator('input[type=file]').setInputFiles(
  'D:/Work/self-project/EnglishASRPlatform/scripts/sample-speech.wav',
)
await page.waitForSelector('table')
await page.click('table a')
await page.waitForSelector('text=Attempts')

const badgeText = () => page.locator('span.capitalize').first().textContent()
console.log('status at load:', (await badgeText())?.trim())

// Deliberately no reload() anywhere below — only the WS-driven query
// invalidation should change what's on screen.
let finalStatus = null
for (let i = 0; i < 20; i++) {
  const status = (await badgeText())?.trim().toLowerCase()
  if (status === 'succeeded' || status === 'failed') {
    finalStatus = status
    break
  }
  await page.waitForTimeout(2000)
}

console.log('status without reload:', finalStatus)
await page.screenshot({ path: 'D:/Work/self-project/EnglishASRPlatform/scripts/live-update-final.png' })
await browser.close()

if (finalStatus !== 'succeeded') {
  console.log('FAIL: status never updated to succeeded without a page reload')
  process.exit(1)
}
console.log('PASS: status updated live via WebSocket, no reload needed')
