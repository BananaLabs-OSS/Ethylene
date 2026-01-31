# demo.ps1 - Ethylene Demo Script

Clear-Host
Write-Host ""
Write-Host "  =============================================" -ForegroundColor Cyan
Write-Host "  ETHYLENE - Binary Patcher Demo" -ForegroundColor Cyan
Write-Host "  Byte-level diffing with manifest versioning" -ForegroundColor Cyan
Write-Host "  =============================================" -ForegroundColor Cyan
Write-Host ""
Start-Sleep -Seconds 3

# Cleanup
Remove-Item -Path "demo_*" -ErrorAction SilentlyContinue
Remove-Item -Path "./patch_demo" -Recurse -ErrorAction SilentlyContinue

Write-Host "STEP 1: Create two versions of a file" -ForegroundColor Yellow
Start-Sleep -Seconds 2

Write-Host ""
Write-Host "  Creating 1 MB file (v1.0.0)..." -ForegroundColor White
$content = "A" * 1000000
$content | Out-File -Encoding ascii demo_old.txt
$oldSize = (Get-Item demo_old.txt).Length
Write-Host "  Done: demo_old.txt" -ForegroundColor Green
Start-Sleep -Seconds 1

Write-Host ""
Write-Host "  Creating 1 MB file (v1.1.0) - changing ONE byte..." -ForegroundColor White
$chars = $content.ToCharArray()
$chars[500000] = "B"
$newContent = -join $chars
$newContent | Out-File -Encoding ascii demo_new.txt
$newSize = (Get-Item demo_new.txt).Length
Write-Host "  Done: demo_new.txt" -ForegroundColor Green
Start-Sleep -Seconds 2

Write-Host ""
Write-Host "  Old file: $([math]::Round($oldSize / 1MB, 2)) MB" -ForegroundColor White
Write-Host "  New file: $([math]::Round($newSize / 1MB, 2)) MB" -ForegroundColor White
Write-Host "  Difference: 1 byte" -ForegroundColor White
Start-Sleep -Seconds 3

Write-Host ""
Write-Host "STEP 2: Generate patch" -ForegroundColor Yellow
Start-Sleep -Seconds 2

Write-Host ""
Write-Host "  Running: ethylene gen --old demo_old.txt --new demo_new.txt" -ForegroundColor Magenta
Write-Host ""
Start-Sleep -Seconds 1

$stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
& go run . gen --old demo_old.txt --new demo_new.txt --out ./patch_demo --from-version 1.0.0 --to-version 1.1.0
$stopwatch.Stop()

Write-Host ""
Write-Host "  Generated in $($stopwatch.ElapsedMilliseconds) ms" -ForegroundColor Cyan
Start-Sleep -Seconds 3

Write-Host ""
Write-Host "STEP 3: Compare sizes" -ForegroundColor Yellow
Start-Sleep -Seconds 2

$patchSize = (Get-Item ./patch_demo/demo_old.txt.patch).Length
$savings = [math]::Round((1 - ($patchSize / $oldSize)) * 100, 2)

Write-Host ""
Write-Host "  Original files: ~1 MB each" -ForegroundColor White
Write-Host "  Patch size:     $patchSize bytes" -ForegroundColor Green
Write-Host "  Reduction:      $savings%" -ForegroundColor Cyan
Start-Sleep -Seconds 4

Write-Host ""
Write-Host "STEP 4: View manifest" -ForegroundColor Yellow
Start-Sleep -Seconds 2

Write-Host ""
Get-Content ./patch_demo/manifest.json
Start-Sleep -Seconds 4

Write-Host ""
Write-Host "STEP 5: Apply patch" -ForegroundColor Yellow
Start-Sleep -Seconds 2

Write-Host ""
Write-Host "  Before: byte 500000 = A" -ForegroundColor Red
Start-Sleep -Seconds 2

Write-Host ""
Write-Host "  Running: ethylene apply --patch ./patch_demo --target ." -ForegroundColor Magenta
Write-Host ""

$stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
& go run . apply --patch ./patch_demo --target .
$stopwatch.Stop()

Write-Host ""
Write-Host "  Applied in $($stopwatch.ElapsedMilliseconds) ms" -ForegroundColor Cyan
Start-Sleep -Seconds 2

$afterByte = (Get-Content demo_old.txt -Raw)[500000]
Write-Host ""
Write-Host "  After: byte 500000 = $afterByte" -ForegroundColor Green
Start-Sleep -Seconds 3

Write-Host ""
Write-Host "=============================================" -ForegroundColor Yellow
Write-Host "RESULT: 1 MB patched with $patchSize bytes" -ForegroundColor Green
Write-Host "=============================================" -ForegroundColor Yellow
Write-Host ""
Start-Sleep -Seconds 5