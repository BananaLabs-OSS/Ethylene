# Ethylene

Cross-platform binary patcher with manifest-based versioning, byte-level diffing, and large file support via HDiffPatch.

From [BananaLabs OSS](https://github.com/bananalabs-oss).

## Overview

Ethylene handles:
- **Patch Generation**: Create byte-level diffs between file versions
- **Patch Application**: Apply patches with hash verification
- **Manifests**: Track versions, checksums, and file actions
- **Directory Walking**: Diff entire folders, only patch what changed
- **Dual Algorithm**: bsdiff for small files, HDiffPatch for large files (multi-GB)

## Quick Start
```bash
go build -o ethylene .

# Single file
./ethylene gen --old ./v1/app.exe --new ./v2/app.exe --out ./patch
./ethylene apply --patch ./patch --target ./installed

# Entire directory
./ethylene gen --old ./game_v1/ --new ./game_v2/ --out ./patch --from-version 1.0.0 --to-version 1.1.0
./ethylene apply --patch ./patch --target ./installed_game/

# Large files — use HDiffPatch
./ethylene gen --old ./v1/ --new ./v2/ --out ./patch --algorithm hdiff
./ethylene apply --patch ./patch --target ./installed/
```

## CLI Reference

### Generate

| Flag | Description | Default |
|------|-------------|---------|
| `--old` | Path to old file or directory | (required) |
| `--new` | Path to new file or directory | (required) |
| `--out` | Output directory for patch | `./patch` |
| `--from-version` | Source version string | `0.0.0` |
| `--to-version` | Target version string | `0.0.1` |
| `--algorithm` | Diff algorithm: `bsdiff` or `hdiff` | `bsdiff` |

```bash
./ethylene gen --old game_v1/ --new game_v2/ --out ./patch --algorithm hdiff --from-version 1.0.0 --to-version 1.1.0
```

### Apply

| Flag | Description | Default |
|------|-------------|---------|
| `--patch` | Path to patch directory | (required) |
| `--target` | Target directory to patch | `.` |

```bash
./ethylene apply --patch ./patch --target ./installed
```

Apply reads the `algorithm` field from the manifest and dispatches to the correct engine automatically.

## Algorithms

| | bsdiff | hdiff |
|---|---|---|
| Implementation | Pure Go | Exec wrapper (hdiffz/hpatchz) |
| Memory | Loads entire file into RAM | Streaming, constant memory |
| Best for | Files under ~500MB | Multi-GB files |
| Speed | Fast for small files | Fast at any size |
| Dependencies | None | Requires hdiffz/hpatchz binaries |

Use `--algorithm bsdiff` (default) for small files. Use `--algorithm hdiff` for anything large.

### HDiffPatch Setup

Download `hdiffz` and `hpatchz` from [HDiffPatch releases](https://github.com/sisong/HDiffPatch/releases) and place them in:

```
ethylene/
  bin/
    windows/
      hdiffz.exe
      hpatchz.exe
    linux/
      hdiffz
      hpatchz
```

Or add them to your system PATH.

## Manifest

Patches include a `manifest.json`:
```json
{
  "from_version": "1.0.0",
  "to_version": "1.1.0",
  "created": "2026-01-31T20:00:00Z",
  "files": [
    {
      "path": "assets/terrain.bin",
      "action": "patch",
      "old_hash": "abc123...",
      "new_hash": "def456...",
      "patch_file": "assets/terrain.bin.patch",
      "algorithm": "hdiff"
    },
    {
      "path": "maps/new_area.dat",
      "action": "add",
      "new_hash": "789abc...",
      "patch_file": "new/maps/new_area.dat"
    },
    {
      "path": "config/old.cfg",
      "action": "delete",
      "old_hash": "321fed..."
    }
  ]
}
```

### Actions

| Action | Description |
|--------|-------------|
| `patch` | Apply binary diff to existing file |
| `add` | Copy new file into target |
| `delete` | Remove file from target (hash verified before removal) |

### Verification

- **Before apply**: Old file hash must match `old_hash`
- **After apply**: New file hash must match `new_hash`
- **On mismatch**: Patch aborts, original file untouched

## Benchmarks

Tested on a 50GB random binary file, WD Black NVMe, streaming SHA256 hashing.

### Small Diff — 2 bytes changed in 50GB

| Metric | Value |
|--------|-------|
| Patch size | **672 bytes** |
| Gen time | 3m 01s |
| Apply time | 1m 34s |

### Large Diff — 1GB changed in 50GB

| Metric | Value |
|--------|-------|
| Patch size | **1,024 MB** |
| Gen time | 11m 13s |
| Apply time | 1m 28s |

Gen runs once on the build server. Apply is ~90 seconds regardless of patch size — bottlenecked by SHA256 hashing at disk speed. bsdiff cannot handle files this large (OOM).

### Distribution Impact

| Scenario | Full re-download | Ethylene patch |
|----------|-----------------|----------------|
| 50GB, 2 bytes changed | 50 GB | 672 bytes |
| 50GB, 1GB changed | 50 GB | 1 GB |
| 10,000 clients, small update | 500 TB served | ~6.5 MB served |

## Directory Example
```bash
./ethylene gen --old game_v1/ --new game_v2/ --out ./patch --algorithm hdiff

# Output:
#   Scanning old directory... (algorithm: hdiff)
#     Found 4 files
#   Scanning new directory...
#     Found 4 files
#   Patch:  assets/player.dat
#   Delete: config/old.cfg
#   Patch:  config/settings.ini
#   Add:    maps/level5.map
#
#   Patch generated (hdiff): 1.0.0 -> 1.1.0
#     2 patched, 1 added, 1 deleted, 1 unchanged

./ethylene apply --patch ./patch --target ./installed_game/
```

## Building

```bash
# Current platform
go build -o ethylene .

# Cross-compile
GOOS=windows GOARCH=amd64 go build -o ethylene.exe .
GOOS=linux GOARCH=amd64 go build -o ethylene .
```

Or use the build script:
```bash
# PowerShell
.\scripts\build.ps1

# Bash
./scripts/build.sh
```

## Use Cases

- **Game updates**: Patch game builds without re-downloading the full client
- **Air-gapped networks**: Distribute patches via USB instead of shipping full builds
- **Terrain/GIS data**: Update massive map datasets by patching only changed tiles
- **CI/CD artifacts**: Distribute incremental build updates to test environments

## Dependencies

- [Potassium](https://github.com/bananalabs-oss/potassium) `diff` — Binary diffing (bsdiff + HDiffPatch)
- [Potassium](https://github.com/bananalabs-oss/potassium) `manifest` — Manifest handling and file hashing
- [HDiffPatch](https://github.com/sisong/HDiffPatch) — Large file diffing (external binaries)

## License

MIT