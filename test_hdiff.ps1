# ============================================================
#  Ethylene HDiffPatch Test
#  Tests both bsdiff and hdiff algorithms side by side
#  Run from the Ethylene project root
# ============================================================

Write-Host ""
Write-Host "=== ETHYLENE HDIFFPATCH TEST ===" -ForegroundColor Cyan
Write-Host ""

# ── Clean up ──────────────────────────────────────────────────
Remove-Item -Recurse -Force test_old, test_new, patch_bsdiff, patch_hdiff, target_bsdiff, target_hdiff -ErrorAction SilentlyContinue

# ── Create test files ─────────────────────────────────────────
Write-Host "Creating test directories..." -ForegroundColor Yellow

New-Item -ItemType Directory -Force test_old, test_new | Out-Null

# Small text file (changed)
"hello world version one" | Out-File -Encoding ascii test_old/small.txt
"hello world version two with extras" | Out-File -Encoding ascii test_new/small.txt

# Larger binary-ish file (1MB, one byte changed)
$bytes = New-Object byte[] 1048576
(New-Object Random 42).NextBytes($bytes)
[IO.File]::WriteAllBytes("$PWD\test_old\big.bin", $bytes)
$bytes[500000] = 255
$bytes[999999] = 128
[IO.File]::WriteAllBytes("$PWD\test_new\big.bin", $bytes)

# Unchanged file
"this stays the same" | Out-File -Encoding ascii test_old/unchanged.txt
"this stays the same" | Out-File -Encoding ascii test_new/unchanged.txt

# New file
"brand new content" | Out-File -Encoding ascii test_new/added.txt

# Deleted file
"gonna be removed" | Out-File -Encoding ascii test_old/removed.txt

Write-Host "  small.txt    (CHANGED)" -ForegroundColor DarkYellow
Write-Host "  big.bin      (1MB, 2 bytes changed)" -ForegroundColor DarkYellow
Write-Host "  unchanged.txt (UNCHANGED)" -ForegroundColor DarkGray
Write-Host "  added.txt    (NEW)" -ForegroundColor Green
Write-Host "  removed.txt  (DELETED)" -ForegroundColor Red

# ── Test 1: bsdiff ────────────────────────────────────────────
Write-Host ""
Write-Host "=== TEST 1: bsdiff ===" -ForegroundColor Magenta
$t1 = Get-Date
go run . gen --old test_old --new test_new --out ./patch_bsdiff --algorithm bsdiff
$t1d = (Get-Date) - $t1
Write-Host "  Gen time: $($t1d.TotalSeconds.ToString('F2'))s" -ForegroundColor DarkCyan

Copy-Item -Recurse test_old target_bsdiff
go run . apply --patch ./patch_bsdiff --target ./target_bsdiff

# ── Test 2: hdiff ─────────────────────────────────────────────
Write-Host ""
Write-Host "=== TEST 2: hdiff ===" -ForegroundColor Magenta
$t2 = Get-Date
go run . gen --old test_old --new test_new --out ./patch_hdiff --algorithm hdiff
$t2d = (Get-Date) - $t2
Write-Host "  Gen time: $($t2d.TotalSeconds.ToString('F2'))s" -ForegroundColor DarkCyan

Copy-Item -Recurse test_old target_hdiff
go run . apply --patch ./patch_hdiff --target ./target_hdiff

# ── Compare patch sizes ──────────────────────────────────────
Write-Host ""
Write-Host "=== PATCH SIZE COMPARISON ===" -ForegroundColor Cyan

$bsdiffSize = (Get-ChildItem -Recurse ./patch_bsdiff -File | Measure-Object -Property Length -Sum).Sum
$hdiffSize = (Get-ChildItem -Recurse ./patch_hdiff -File | Measure-Object -Property Length -Sum).Sum

Write-Host "  bsdiff total: $bsdiffSize bytes" -ForegroundColor White
Write-Host "  hdiff total:  $hdiffSize bytes" -ForegroundColor White

Write-Host ""
Write-Host "  bsdiff patches:" -ForegroundColor DarkGray
Get-ChildItem -Recurse ./patch_bsdiff -File | ForEach-Object {
    $rel = $_.FullName.Replace((Get-Location).Path + '\patch_bsdiff\', '')
    Write-Host "    $rel  ($($_.Length) bytes)" -ForegroundColor White
}

Write-Host ""
Write-Host "  hdiff patches:" -ForegroundColor DarkGray
Get-ChildItem -Recurse ./patch_hdiff -File | ForEach-Object {
    $rel = $_.FullName.Replace((Get-Location).Path + '\patch_hdiff\', '')
    Write-Host "    $rel  ($($_.Length) bytes)" -ForegroundColor White
}

# ── Verify both produced the same result ──────────────────────
Write-Host ""
Write-Host "=== VERIFICATION ===" -ForegroundColor Green

$pass = $true

# Check small.txt
$bs = Get-Content ./target_bsdiff/small.txt -Raw
$hd = Get-Content ./target_hdiff/small.txt -Raw
if ($bs -eq $hd) {
    Write-Host "  small.txt:     MATCH" -ForegroundColor Green
} else {
    Write-Host "  small.txt:     MISMATCH" -ForegroundColor Red
    $pass = $false
}

# Check big.bin via hash
$bsHash = (Get-FileHash ./target_bsdiff/big.bin).Hash
$hdHash = (Get-FileHash ./target_hdiff/big.bin).Hash
$newHash = (Get-FileHash ./test_new/big.bin).Hash
if ($bsHash -eq $newHash -and $hdHash -eq $newHash) {
    Write-Host "  big.bin:       MATCH (both match expected)" -ForegroundColor Green
} else {
    Write-Host "  big.bin:       MISMATCH" -ForegroundColor Red
    Write-Host "    expected: $($newHash.Substring(0,16))..." -ForegroundColor Red
    Write-Host "    bsdiff:   $($bsHash.Substring(0,16))..." -ForegroundColor Red
    Write-Host "    hdiff:    $($hdHash.Substring(0,16))..." -ForegroundColor Red
    $pass = $false
}

# Check added.txt exists
$bsAdded = Test-Path ./target_bsdiff/added.txt
$hdAdded = Test-Path ./target_hdiff/added.txt
if ($bsAdded -and $hdAdded) {
    Write-Host "  added.txt:     EXISTS in both" -ForegroundColor Green
} else {
    Write-Host "  added.txt:     MISSING" -ForegroundColor Red
    $pass = $false
}

# Check removed.txt gone
$bsRemoved = Test-Path ./target_bsdiff/removed.txt
$hdRemoved = Test-Path ./target_hdiff/removed.txt
if (-not $bsRemoved -and -not $hdRemoved) {
    Write-Host "  removed.txt:   DELETED in both" -ForegroundColor Green
} else {
    Write-Host "  removed.txt:   STILL EXISTS" -ForegroundColor Red
    $pass = $false
}

# Check unchanged
$bsUnch = Test-Path ./target_bsdiff/unchanged.txt
$hdUnch = Test-Path ./target_hdiff/unchanged.txt
if ($bsUnch -and $hdUnch) {
    Write-Host "  unchanged.txt: EXISTS in both" -ForegroundColor Green
} else {
    Write-Host "  unchanged.txt: MISSING" -ForegroundColor Red
    $pass = $false
}

# Check manifest algorithm field
Write-Host ""
Write-Host "  bsdiff manifest algorithm:" -ForegroundColor DarkGray
$bsManifest = Get-Content ./patch_bsdiff/manifest.json | ConvertFrom-Json
$bsManifest.files | Where-Object { $_.algorithm } | ForEach-Object {
    Write-Host "    $($_.path) -> $($_.algorithm)" -ForegroundColor White
}

Write-Host "  hdiff manifest algorithm:" -ForegroundColor DarkGray
$hdManifest = Get-Content ./patch_hdiff/manifest.json | ConvertFrom-Json
$hdManifest.files | Where-Object { $_.algorithm } | ForEach-Object {
    Write-Host "    $($_.path) -> $($_.algorithm)" -ForegroundColor White
}

Write-Host ""
if ($pass) {
    Write-Host "=== ALL TESTS PASSED ===" -ForegroundColor Green
} else {
    Write-Host "=== SOME TESTS FAILED ===" -ForegroundColor Red
}

# ── Cleanup ───────────────────────────────────────────────────
# Uncomment to auto-clean:
Remove-Item -Recurse -Force test_old, test_new, patch_bsdiff, patch_hdiff, target_bsdiff, target_hdiff