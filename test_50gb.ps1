# ============================================================
#  Ethylene 50GB Stress Test
#  Creates a 50GB file, makes small and large diffs, benchmarks
#  Run from the Ethylene project root
#
#  WARNING: Needs ~200GB free disk (50GB x4 copies)
#  WARNING: bsdiff WILL crash or take forever on 50GB. That's the point.
# ============================================================

param(
    [long]$SizeGB = 50,
    [switch]$SkipBsdiff  # Use this if you don't want to wait for bsdiff to OOM
)

$SizeBytes = $SizeGB * 1GB

Write-Host ""
Write-Host "=== ETHYLENE 50GB STRESS TEST ===" -ForegroundColor Cyan
Write-Host "  File size: ${SizeGB}GB ($SizeBytes bytes)" -ForegroundColor DarkCyan
Write-Host ""

# ── Clean up ──────────────────────────────────────────────────
Remove-Item -Recurse -Force stress_old, stress_new, stress_patch_bsdiff, stress_patch_hdiff, stress_target_hdiff -ErrorAction SilentlyContinue

New-Item -ItemType Directory -Force stress_old, stress_new | Out-Null

# ── Generate base file ────────────────────────────────────────
Write-Host "Generating ${SizeGB}GB base file..." -ForegroundColor Yellow
Write-Host "  (This will take a few minutes)" -ForegroundColor DarkGray

$sw = [System.Diagnostics.Stopwatch]::StartNew()

# Write in 64MB chunks for speed
$chunkSize = 64 * 1024 * 1024  # 64MB
$chunk = New-Object byte[] $chunkSize
$rng = New-Object System.Random(42)

$stream = [System.IO.File]::Create("$PWD\stress_old\game_assets.bin")
$written = 0L
while ($written -lt $SizeBytes) {
    $remaining = $SizeBytes - $written
    $toWrite = [Math]::Min([long]$chunkSize, [long]$remaining)

    if ($toWrite -lt $chunkSize) {
        $chunk = [byte[]]::new([int]$toWrite)
    }
    $rng.NextBytes($chunk)
    $stream.Write($chunk, 0, $toWrite)
    $written += $toWrite

    $pct = [Math]::Round(($written / $SizeBytes) * 100, 1)
    $gbDone = [Math]::Round($written / 1GB, 2)
    Write-Host "`r  ${gbDone}GB / ${SizeGB}GB  (${pct}%)" -NoNewline -ForegroundColor DarkYellow
}
$stream.Close()
Write-Host ""

$sw.Stop()
Write-Host "  Base file created in $($sw.Elapsed.TotalSeconds.ToString('F1'))s" -ForegroundColor Green

# ── Create SMALL diff version (2 bytes changed) ──────────────
Write-Host ""
Write-Host "Creating SMALL diff version (2 bytes changed in 50GB)..." -ForegroundColor Yellow

Copy-Item "stress_old\game_assets.bin" "stress_new\game_assets.bin"

# Flip 2 bytes at known offsets
$stream = [System.IO.File]::Open("$PWD\stress_new\game_assets.bin", 'Open', 'ReadWrite')
$stream.Seek(1000000, 'Begin') | Out-Null
$stream.WriteByte(0xFF)
$stream.Seek($SizeBytes - 1000, 'Begin') | Out-Null
$stream.WriteByte(0x42)
$stream.Close()

Write-Host "  Done - 2 bytes flipped" -ForegroundColor Green

# ══════════════════════════════════════════════════════════════
#  TEST A: SMALL DIFF (2 bytes changed)
# ══════════════════════════════════════════════════════════════

Write-Host ""
Write-Host "══════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "  TEST A: SMALL DIFF (2 bytes in ${SizeGB}GB)" -ForegroundColor Cyan
Write-Host "══════════════════════════════════════════" -ForegroundColor Cyan

# ── hdiff: small diff ─────────────────────────────────────────
Write-Host ""
Write-Host "--- hdiff gen (small diff) ---" -ForegroundColor Magenta
$t = [System.Diagnostics.Stopwatch]::StartNew()
go run . gen --old stress_old --new stress_new --out ./stress_patch_hdiff --algorithm hdiff
$t.Stop()
$hdiffGenSmall = $t.Elapsed
Write-Host "  hdiff gen time: $($hdiffGenSmall.ToString())" -ForegroundColor Cyan

$hdiffPatchSize = (Get-Item ./stress_patch_hdiff/game_assets.bin.patch).Length
Write-Host "  hdiff patch size: $hdiffPatchSize bytes" -ForegroundColor Cyan

Write-Host ""
Write-Host "--- hdiff apply (small diff) ---" -ForegroundColor Magenta
New-Item -ItemType Directory -Force stress_target_hdiff | Out-Null
Copy-Item "stress_old\game_assets.bin" "stress_target_hdiff\game_assets.bin"

$t = [System.Diagnostics.Stopwatch]::StartNew()
go run . apply --patch ./stress_patch_hdiff --target ./stress_target_hdiff
$t.Stop()
$hdiffApplySmall = $t.Elapsed
Write-Host "  hdiff apply time: $($hdiffApplySmall.ToString())" -ForegroundColor Cyan

# Verify
$expectedHash = (Get-FileHash ./stress_new/game_assets.bin -Algorithm SHA256).Hash
$actualHash = (Get-FileHash ./stress_target_hdiff/game_assets.bin -Algorithm SHA256).Hash
if ($expectedHash -eq $actualHash) {
    Write-Host "  hdiff verify: PASS" -ForegroundColor Green
} else {
    Write-Host "  hdiff verify: FAIL" -ForegroundColor Red
}

# ── bsdiff: small diff ───────────────────────────────────────
if (-not $SkipBsdiff) {
    Write-Host ""
    Write-Host "--- bsdiff gen (small diff) ---" -ForegroundColor Magenta
    Write-Host "  (This may OOM or take a very long time...)" -ForegroundColor Red

    Remove-Item -Recurse -Force stress_patch_bsdiff -ErrorAction SilentlyContinue

    $t = [System.Diagnostics.Stopwatch]::StartNew()
    go run . gen --old stress_old --new stress_new --out ./stress_patch_bsdiff --algorithm bsdiff 2>&1
    $bsdiffExit = $LASTEXITCODE
    $t.Stop()
    $bsdiffGenSmall = $t.Elapsed

    if ($bsdiffExit -ne 0) {
        Write-Host "  bsdiff CRASHED (expected for ${SizeGB}GB)" -ForegroundColor Red
        Write-Host "  bsdiff gen time before crash: $($bsdiffGenSmall.ToString())" -ForegroundColor DarkRed
    } else {
        Write-Host "  bsdiff gen time: $($bsdiffGenSmall.ToString())" -ForegroundColor Cyan
        $bsdiffPatchSize = (Get-Item ./stress_patch_bsdiff/game_assets.bin.patch).Length
        Write-Host "  bsdiff patch size: $bsdiffPatchSize bytes" -ForegroundColor Cyan
    }
} else {
    Write-Host ""
    Write-Host "--- bsdiff: SKIPPED (--SkipBsdiff) ---" -ForegroundColor DarkGray
}

# ══════════════════════════════════════════════════════════════
#  TEST B: LARGE DIFF (1GB changed)
# ══════════════════════════════════════════════════════════════

Write-Host ""
Write-Host "══════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "  TEST B: LARGE DIFF (1GB changed in ${SizeGB}GB)" -ForegroundColor Cyan
Write-Host "══════════════════════════════════════════" -ForegroundColor Cyan

Write-Host ""
Write-Host "Creating LARGE diff version (overwriting 1GB region)..." -ForegroundColor Yellow

# Overwrite a 1GB region in the middle of the new file
Copy-Item "stress_old\game_assets.bin" "stress_new\game_assets.bin" -Force

$stream = [System.IO.File]::Open("$PWD\stress_new\game_assets.bin", 'Open', 'ReadWrite')
$overwriteSize = 1GB
$overwriteOffset = [long]($SizeBytes / 2)  # middle of file
$stream.Seek($overwriteOffset, 'Begin') | Out-Null

$overwriteChunk = New-Object byte[] (64 * 1024 * 1024)  # 64MB at a time
$rng2 = New-Object System.Random(99)  # different seed = different data
$overwritten = 0L
while ($overwritten -lt $overwriteSize) {
    $remaining = $overwriteSize - $overwritten
    $toWrite = [Math]::Min([long](64 * 1024 * 1024), [long]$remaining)
    if ($toWrite -lt $overwriteChunk.Length) {
        $overwriteChunk = [byte[]]::new([int]$toWrite)
    }
    $rng2.NextBytes($overwriteChunk)
    $stream.Write($overwriteChunk, 0, $toWrite)
    $overwritten += $toWrite

    $pct = [Math]::Round(($overwritten / $overwriteSize) * 100, 1)
    Write-Host "`r  Overwriting: ${pct}%" -NoNewline -ForegroundColor DarkYellow
}
$stream.Close()
Write-Host ""
Write-Host "  Done - 1GB overwritten at offset $overwriteOffset" -ForegroundColor Green

# Clean previous patches
Remove-Item -Recurse -Force stress_patch_hdiff, stress_target_hdiff -ErrorAction SilentlyContinue

# ── hdiff: large diff ─────────────────────────────────────────
Write-Host ""
Write-Host "--- hdiff gen (large diff) ---" -ForegroundColor Magenta
$t = [System.Diagnostics.Stopwatch]::StartNew()
go run . gen --old stress_old --new stress_new --out ./stress_patch_hdiff --algorithm hdiff
$t.Stop()
$hdiffGenLarge = $t.Elapsed
Write-Host "  hdiff gen time: $($hdiffGenLarge.ToString())" -ForegroundColor Cyan

$hdiffPatchSizeLarge = (Get-Item ./stress_patch_hdiff/game_assets.bin.patch).Length
$hdiffPatchMB = [Math]::Round($hdiffPatchSizeLarge / 1MB, 2)
Write-Host "  hdiff patch size: ${hdiffPatchMB}MB ($hdiffPatchSizeLarge bytes)" -ForegroundColor Cyan

Write-Host ""
Write-Host "--- hdiff apply (large diff) ---" -ForegroundColor Magenta
New-Item -ItemType Directory -Force stress_target_hdiff | Out-Null
Copy-Item "stress_old\game_assets.bin" "stress_target_hdiff\game_assets.bin"

$t = [System.Diagnostics.Stopwatch]::StartNew()
go run . apply --patch ./stress_patch_hdiff --target ./stress_target_hdiff
$t.Stop()
$hdiffApplyLarge = $t.Elapsed
Write-Host "  hdiff apply time: $($hdiffApplyLarge.ToString())" -ForegroundColor Cyan

# Verify
$expectedHash = (Get-FileHash ./stress_new/game_assets.bin -Algorithm SHA256).Hash
$actualHash = (Get-FileHash ./stress_target_hdiff/game_assets.bin -Algorithm SHA256).Hash
if ($expectedHash -eq $actualHash) {
    Write-Host "  hdiff verify: PASS" -ForegroundColor Green
} else {
    Write-Host "  hdiff verify: FAIL" -ForegroundColor Red
}

# ══════════════════════════════════════════════════════════════
#  SUMMARY
# ══════════════════════════════════════════════════════════════

Write-Host ""
Write-Host "══════════════════════════════════════════" -ForegroundColor Green
Write-Host "  RESULTS SUMMARY" -ForegroundColor Green
Write-Host "══════════════════════════════════════════" -ForegroundColor Green
Write-Host ""
Write-Host "  File size: ${SizeGB}GB" -ForegroundColor White
Write-Host ""
Write-Host "  SMALL DIFF (2 bytes changed):" -ForegroundColor Yellow
Write-Host "    hdiff gen:   $($hdiffGenSmall.ToString())" -ForegroundColor Cyan
Write-Host "    hdiff apply: $($hdiffApplySmall.ToString())" -ForegroundColor Cyan
Write-Host "    hdiff patch: $hdiffPatchSize bytes" -ForegroundColor Cyan
if (-not $SkipBsdiff) {
    if ($bsdiffExit -ne 0) {
        Write-Host "    bsdiff:      CRASHED" -ForegroundColor Red
    } else {
        Write-Host "    bsdiff gen:  $($bsdiffGenSmall.ToString())" -ForegroundColor DarkYellow
        Write-Host "    bsdiff patch: $bsdiffPatchSize bytes" -ForegroundColor DarkYellow
    }
}
Write-Host ""
Write-Host "  LARGE DIFF (1GB changed):" -ForegroundColor Yellow
Write-Host "    hdiff gen:   $($hdiffGenLarge.ToString())" -ForegroundColor Cyan
Write-Host "    hdiff apply: $($hdiffApplyLarge.ToString())" -ForegroundColor Cyan
Write-Host "    hdiff patch: ${hdiffPatchMB}MB" -ForegroundColor Cyan
Write-Host ""

# ── Cleanup reminder ──────────────────────────────────────────
$totalSize = [Math]::Round(((Get-ChildItem -Recurse stress_old, stress_new, stress_patch_hdiff, stress_target_hdiff -ErrorAction SilentlyContinue | Measure-Object -Property Length -Sum).Sum) / 1GB, 2)
Write-Host "  Disk used by test files: ~${totalSize}GB" -ForegroundColor DarkGray
Write-Host "  Run this to clean up:" -ForegroundColor DarkGray
Write-Host "    Remove-Item -Recurse -Force stress_old, stress_new, stress_patch_bsdiff, stress_patch_hdiff, stress_target_hdiff" -ForegroundColor DarkGray