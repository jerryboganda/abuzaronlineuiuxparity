param(
    [string]$Url = 'http://127.0.0.1:5173/login',
    [int]$Iterations = 5,
    [string]$OutputPath = ''
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$webRoot = Join-Path $root 'apps\web'
if ($Iterations -lt 3 -or $Iterations -gt 20) {
    throw 'Iterations must be between 3 and 20.'
}
if ([string]::IsNullOrWhiteSpace($OutputPath)) {
    $OutputPath = Join-Path $root ('tmp\phase-w-browser-cold-start-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.json')
}

$nodeScript = @'
const { chromium } = require("@playwright/test");
(async () => {
  const url = process.env.PHASE_W_BROWSER_URL;
  const iterations = Number(process.env.PHASE_W_BROWSER_ITERATIONS);
  const samples = [];
  for (let i = 0; i < iterations; i += 1) {
    const started = process.hrtime.bigint();
    const browser = await chromium.launch({ headless: true });
    const page = await browser.newPage({ viewport: { width: 1936, height: 1048 } });
    await page.goto(url, { waitUntil: "domcontentloaded", timeout: 10000 });
    await page.locator("main").waitFor({ state: "visible", timeout: 10000 });
    await browser.close();
    samples.push(Number(process.hrtime.bigint() - started) / 1000000);
  }
  samples.sort((a, b) => a - b);
  const percentile = (p) => samples[Math.floor((samples.length - 1) * p)];
  process.stdout.write(JSON.stringify({
    measuredAt: new Date().toISOString(),
    url,
    iterations,
    samplesMs: samples,
    p50Ms: percentile(0.5),
    p95Ms: percentile(0.95),
    budgetMs: 3000,
    acceptance: percentile(0.95) < 3000 ? "observed_green_browser_probe" : "pending_review"
  }, null, 2));
})().catch((error) => {
  process.stderr.write(String(error && error.stack || error));
  process.exit(1);
});
'@

$env:PHASE_W_BROWSER_URL = $Url
$env:PHASE_W_BROWSER_ITERATIONS = [string]$Iterations
Push-Location $webRoot
$result = $nodeScript | node -
Pop-Location
if ($LASTEXITCODE -ne 0) {
    throw 'Browser cold-start probe failed. Ensure the web dependencies and local web server are available.'
}
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $OutputPath) | Out-Null
$result | Set-Content -LiteralPath $OutputPath -Encoding UTF8
Remove-Item Env:PHASE_W_BROWSER_URL -ErrorAction SilentlyContinue
Remove-Item Env:PHASE_W_BROWSER_ITERATIONS -ErrorAction SilentlyContinue
Write-Output $result
