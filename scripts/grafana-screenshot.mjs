// Manual verification aid — screenshots the ASR Platform Grafana
// dashboard, e.g. for a README update after a load test run.
//
// Setup: npm install playwright && npx playwright install chromium
// Run:   kubectl -n asr-platform port-forward svc/grafana 3000:3000 &
//        node scripts/grafana-screenshot.mjs
//
// waitUntil: 'networkidle' + a few seconds' wait matters here — Grafana's
// panel grid (react-grid-layout) can report zero-height panels to a
// screenshot taken before its post-load layout pass finishes, even
// though the panels are already correctly positioned in the DOM by then.
import { chromium } from 'playwright'

const GRAFANA_URL = process.env.GRAFANA_URL || 'http://localhost:3000'
const OUT_PATH = process.env.OUT_PATH || './screenshots/grafana-dashboard.png'

const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 1400, height: 1000 } })

await page.goto(`${GRAFANA_URL}/d/asr-platform/asr-platform?orgId=1&from=now-15m&to=now`, {
  waitUntil: 'networkidle',
})
await page.waitForTimeout(5000)
await page.screenshot({ path: OUT_PATH })
console.log('saved', OUT_PATH)

await browser.close()
