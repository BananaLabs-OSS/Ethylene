# Ethylene

Cross-platform binary patcher with manifest-based versioning and byte-level diffing.

From [BananaLabs OSS](https://github.com/bananalabs-oss).

## Overview

Ethylene handles:
- **Patch Generation**: Create byte-level diffs between file versions
- **Patch Application**: Apply patches with hash verification
- **Manifests**: Track versions, checksums, and file actions

## Quick Start
```bash
go build
./ethylene gen --old ./v1/app.exe --new ./v2/app.exe --out ./patch
./ethylene apply --patch ./patch --target ./installed
```

## CLI Reference

### Generate

| Flag | Description | Default |
|------|-------------|---------|
| `--old` | Path to old file | (required) |
| `--new` | Path to new file | (required) |
| `--out` | Output directory for patch | `./patch` |
| `--from-version` | Source version string | `0.0.0` |
| `--to-version` | Target version string | `0.0.1` |
```bash
./ethylene gen --old game.exe --new game_v2.exe --out ./patch --from-version 1.0.0 --to-version 1.1.0
```

### Apply

| Flag | Description | Default |
|------|-------------|---------|
| `--patch` | Path to patch directory | (required) |
| `--target` | Target directory to patch | `.` |
```bash
./ethylene apply --patch ./patch --target ./installed
```

## Manifest

Patches include a `manifest.json`:
```json
{
  "from_version": "1.0.0",
  "to_version": "1.1.0",
  "created": "2026-01-30T20:00:00Z",
  "files": [
    {
      "path": "game.exe",
      "action": "patch",
      "old_hash": "abc123...",
      "new_hash": "def456...",
      "patch_file": "game.exe.patch"
    }
  ]
}
```

### Actions

| Action | Description |
|--------|-------------|
| `patch` | Apply binary diff to existing file |
| `add` | Add new file (not yet implemented) |
| `delete` | Remove file |

### Verification

- **Before apply**: Old file hash must match `old_hash`
- **After apply**: New file hash must match `new_hash`
- **On mismatch**: Patch fails, original file unchanged

## Example
```bash
# Two 1 MB files, 1 byte different
echo "AAAA..." > old.txt   # 1 MB of A's
echo "AAAB..." > new.txt   # 1 byte changed to B

# Generate patch
./ethylene gen --old old.txt --new new.txt --out ./patch

# Patch size: ~156 bytes (not 1 MB)

# Apply
./ethylene apply --patch ./patch --target .
```

## Dependencies

- [Potassium](https://github.com/bananalabs-oss/potassium) - Diff and manifest libraries

## License

MIT