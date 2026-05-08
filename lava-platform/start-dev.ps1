$root = $PSScriptRoot

Write-Host "=== Lava Crash Dev Stack ===" -ForegroundColor Cyan

# 1. Kill existing processes on ports 3000 and 8080
foreach ($port in @(3000, 8080)) {
    $pids = (netstat -ano | Select-String ":$port " | Select-String "LISTENING" | ForEach-Object { ($_ -split "\s+")[-1] })
    foreach ($p in $pids) { if ($p -match '^\d+$') { Stop-Process -Id $p -Force -ErrorAction SilentlyContinue } }
}
Get-Process | Where-Object { $_.ProcessName -like "*cloudflared*" } | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep 2

# 2. Start Go backend in new window
Write-Host "[1/3] Starting Go backend..." -ForegroundColor Yellow
Start-Process powershell -ArgumentList "-NoExit", "-Command", "Set-Location '$root'; go run ./cmd/api/..." -WindowStyle Normal

Start-Sleep 5

# 3. Start Vite frontend in new window
Write-Host "[2/3] Starting Vite frontend..." -ForegroundColor Yellow
Start-Process powershell -ArgumentList "-NoExit", "-Command", "Set-Location '$root\frontend'; npm run dev" -WindowStyle Normal

Start-Sleep 5

# 4. Start Cloudflare tunnel and capture URL
Write-Host "[3/3] Starting Cloudflare tunnel..." -ForegroundColor Yellow
$ngrokUrl = "https://affirm-backstab-diffusion.ngrok-free.dev"

Write-Host "[3/3] Starting ngrok tunnel ($ngrokUrl)..." -ForegroundColor Yellow
Start-Process powershell -ArgumentList "-NoExit", "-Command", "ngrok http 3000 --url=affirm-backstab-diffusion.ngrok-free.dev" -WindowStyle Normal
Start-Sleep 5

# Update .env
$envPath = "$root\.env"
(Get-Content $envPath) -replace "TELEGRAM_APP_URL=.*", "TELEGRAM_APP_URL=$ngrokUrl" | Set-Content $envPath
Write-Host "✅ .env updated" -ForegroundColor Green

# Update Telegram menu button
$token = (Get-Content $envPath | Select-String "TELEGRAM_BOT_TOKEN=(.+)").Matches.Groups[1].Value
$body = "{`"menu_button`":{`"type`":`"web_app`",`"text`":`"Play Now`",`"web_app`":{`"url`":`"$ngrokUrl`"}}}"
$result = Invoke-RestMethod -Uri "https://api.telegram.org/bot$token/setChatMenuButton" -Method Post -Body $body -ContentType "application/json"
if ($result.ok) { Write-Host "✅ Telegram menu button updated" -ForegroundColor Green }
else { Write-Host "⚠️  Telegram update failed" -ForegroundColor Red }

Write-Host ""
Write-Host "=== ALL SERVICES RUNNING ===" -ForegroundColor Cyan
Write-Host "Frontend : http://localhost:3000" -ForegroundColor White
Write-Host "Backend  : http://localhost:8080" -ForegroundColor White
Write-Host "Public   : $ngrokUrl" -ForegroundColor White
Write-Host ""
Write-Host "Send /start to the bot in Telegram to play." -ForegroundColor Yellow
