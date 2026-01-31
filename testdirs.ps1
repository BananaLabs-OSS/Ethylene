# ============================================================
#  Ethylene Directory Walking Test
#  Run from the Ethylene project root
# ============================================================

Write-Host ""
Write-Host "=== ETHYLENE DIRECTORY TEST ===" -ForegroundColor Cyan
Write-Host ""

# ── Clean up any previous test ────────────────────────────────
Remove-Item -Recurse -Force game_v1, game_v2, patch_dir, target_game -ErrorAction SilentlyContinue

# ── Create v1 (the "installed" game) ─────────────────────────
Write-Host "Creating game_v1/ ..." -ForegroundColor Yellow
New-Item -ItemType Directory -Force game_v1/assets, game_v1/config | Out-Null

"player data version one"      | Out-File -Encoding ascii game_v1/assets/player.dat
"enemy data shared"            | Out-File -Encoding ascii game_v1/assets/enemy.dat
"graphics=low`naudio=high"     | Out-File -Encoding ascii game_v1/config/settings.ini
"this file will be removed"    | Out-File -Encoding ascii game_v1/config/old.cfg

Write-Host "  assets/player.dat   (will CHANGE)"  -ForegroundColor White
Write-Host "  assets/enemy.dat    (UNCHANGED)"     -ForegroundColor DarkGray
Write-Host "  config/settings.ini (will CHANGE)"   -ForegroundColor White
Write-Host "  config/old.cfg      (will DELETE)"   -ForegroundColor Red

# ── Create v2 (the new version) ──────────────────────────────
Write-Host ""
Write-Host "Creating game_v2/ ..." -ForegroundColor Yellow
New-Item -ItemType Directory -Force game_v2/assets, game_v2/config, game_v2/maps | Out-Null

"player data version two with new abilities and stats" | Out-File -Encoding ascii game_v2/assets/player.dat
"enemy data shared"            | Out-File -Encoding ascii game_v2/assets/enemy.dat
"graphics=ultra`naudio=high`nfps_cap=144" | Out-File -Encoding ascii game_v2/config/settings.ini
# old.cfg is GONE
"level five map binary data here" | Out-File -Encoding ascii game_v2/maps/level5.map

Write-Host "  assets/player.dat   (CHANGED)"   -ForegroundColor DarkYellow
Write-Host "  assets/enemy.dat    (UNCHANGED)"  -ForegroundColor DarkGray
Write-Host "  config/settings.ini (CHANGED)"    -ForegroundColor DarkYellow
Write-Host "  config/old.cfg      (DELETED)"    -ForegroundColor Red
Write-Host "  maps/level5.map     (NEW)"        -ForegroundColor Green

# ── Generate patch ────────────────────────────────────────────
Write-Host ""
Write-Host "Generating patch..." -ForegroundColor Cyan
$genStart = Get-Date
go run . gen --old game_v1 --new game_v2 --out ./patch_dir --from-version 1.0.0 --to-version 1.1.0
$genTime = (Get-Date) - $genStart
Write-Host "  Gen time: $($genTime.TotalSeconds.ToString('F2'))s" -ForegroundColor DarkCyan

# ── Show patch bundle ─────────────────────────────────────────
Write-Host ""
Write-Host "Patch bundle contents:" -ForegroundColor Cyan
Get-ChildItem -Recurse ./patch_dir | ForEach-Object {
    $size = if ($_.PSIsContainer) { "" } else { "($($_.Length) bytes)" }
    Write-Host "  $($_.FullName.Replace((Get-Location).Path + '\patch_dir\', ''))  $size" -ForegroundColor White
}

# ── Show manifest ─────────────────────────────────────────────
Write-Host ""
Write-Host "Manifest:" -ForegroundColor Cyan
Get-Content ./patch_dir/manifest.json

# ── Copy v1 as the "installed" target ─────────────────────────
Write-Host ""
Write-Host "Copying game_v1/ -> target_game/ (simulating installed game)" -ForegroundColor Yellow
Copy-Item -Recurse game_v1 target_game

# ── Apply patch ───────────────────────────────────────────────
Write-Host ""
Write-Host "Applying patch to target_game/..." -ForegroundColor Cyan
$applyStart = Get-Date
go run . apply --patch ./patch_dir --target ./target_game
$applyTime = (Get-Date) - $applyStart
Write-Host "  Apply time: $($applyTime.TotalSeconds.ToString('F2'))s" -ForegroundColor DarkCyan

# ── Verify results ────────────────────────────────────────────
Write-Host ""
Write-Host "=== VERIFICATION ===" -ForegroundColor Green

Write-Host ""
Write-Host "target_game/ contents:" -ForegroundColor White
Get-ChildItem -Recurse ./target_game -File | ForEach-Object {
    $rel = $_.FullName.Replace((Get-Location).Path + '\target_game\', '')
    Write-Host "  $rel  ($($_.Length) bytes)" -ForegroundColor White
}

Write-Host ""
Write-Host "Checking player.dat was patched:" -ForegroundColor White
Write-Host "  $(Get-Content ./target_game/assets/player.dat)" -ForegroundColor Green

Write-Host ""
Write-Host "Checking settings.ini was patched:" -ForegroundColor White
Get-Content ./target_game/config/settings.ini | ForEach-Object { Write-Host "  $_" -ForegroundColor Green }

Write-Host ""
$oldCfgExists = Test-Path ./target_game/config/old.cfg
Write-Host "Checking old.cfg was deleted: $(if (-not $oldCfgExists) { 'YES - gone!' } else { 'NO - still exists!' })" -ForegroundColor $(if (-not $oldCfgExists) { "Green" } else { "Red" })

Write-Host ""
$mapExists = Test-Path ./target_game/maps/level5.map
Write-Host "Checking level5.map was added: $(if ($mapExists) { 'YES - exists!' } else { 'NO - missing!' })" -ForegroundColor $(if ($mapExists) { "Green" } else { "Red" })
if ($mapExists) {
    Write-Host "  $(Get-Content ./target_game/maps/level5.map)" -ForegroundColor Green
}

Write-Host ""
Write-Host "=== DONE ===" -ForegroundColor Cyan

# ── Cleanup ───────────────────────────────────────────────────
# Uncomment to auto-clean:
Remove-Item -Recurse -Force game_v1, game_v2, patch_dir, target_game