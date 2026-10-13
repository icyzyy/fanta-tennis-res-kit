# fanta-tennis-res-kit

[![Go Version](https://img.shields.io/badge/go-1.27%2B-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/S4V328HBWR)

`fanta-tennis-res-kit` is a Go library and command-line toolkit for inspecting, modifying, and reverse-engineering FantaTennis client archive files (`Res/*.res`). It provides tools to browse archives, decrypt `.set` configuration files, edit and repack archives with atomic integrity guarantees, search entries across archives, batch extract contents, and decode/encode compressed `.tex` game textures as well as the `.dds`/`.bmp`/`.tga` image leftovers.

---

## Features

- **Archive Management (`.res`)**:
  - Full read, listing, and inspection of game archives (ZIP container format).
  - Atomic archive updates (`Replace`) ensuring zero corruption, with automatic backup and rollback support.
- **`.set` Cryptography**:
  - AES-128-ECB cipher handling the game's custom 1-byte padding scheme (`pad = (16 - len % 16) % 16`).
  - Decrypt `.set` XML configuration files directly to plaintext.
  - Encrypt modified XML files back to valid game `.set` files.
  - Strict XML validation on saving to prevent crashing the game client with malformed documents.
- **`.tex` Texture Decoding & Encoding**:
  - Inverts the 128-byte DDS header only (the game loader's transform); the pixel body is read/written raw.
  - DirectDraw Surface (DDS) block decompression: DXT1 (auto-detecting opaque vs 1-bit alpha punch-through), DXT3, DXT5, and uncompressed RGB24 / RGBA32 formats to PNG.
  - DDS texture encoder: DXT1/3/5 and uncompressed RGB24/RGBA32 with Euclidean max-distance endpoint selection, alpha-weighted box downsampling, and full Direct3D mip chains.
- **Image Leftovers (`.dds` / `.bmp` / `.tga`)**:
  - `adapter/imgdecoder` dispatches on the entry extension and composes the existing `.tex` decoder; exposed through the `usecase.ImageDecoder` port and `NewPreviewImage` interactor (no `KindOf` gate — extension dispatch lives in the adapter).
  - `.dds`: plain standard DDS — the first 128 bytes are inverted to the `.tex` container shape, then decoded by `texdecoder`.
  - `.bmp`: classic BITMAPINFOHEADER via `golang.org/x/image/bmp`.
  - `.tga`: own decoder — color-mapped/truecolor/grayscale (types 1/2/3) plus RLE (9/10/11), 15/16/24/32-bit color maps, 8/15/16/24/32-bit depths, origin handling, strict errors on malformed input.
- **Fast Search**:
  - Streamed, cancellable search across multiple archives with `mtime`-aware entry caching.
- **Clean Architecture**:
  - Strict dependency boundaries: `domain` &rarr; `usecase` &rarr; `adapter` &rarr; `driver / cmd`.
  - One focused third-party dependency (`golang.org/x/image`, for BMP decoding); everything else is the Go standard library.

---

## Installation & Build

### Requirements
- Go 1.27 or newer

### Installation as a Go Module
```bash
go get github.com/icyzyy/fanta-tennis-res-kit
```

### Building the CLI from Source
```bash
git clone https://github.com/icyzyy/fanta-tennis-res-kit.git
cd fanta-tennis-res-kit
go build -o fanta-tennis-res-kit ./cmd/fanta-tennis-res-kit
```

---

## CLI Usage

Run `fanta-tennis-res-kit` or `fanta-tennis-res-kit help` to inspect available commands:

```text
usage: fanta-tennis-res-kit <command> [flags] [args]

commands:
  decrypt  [-out FILE] ARCHIVE ENTRY     decrypt a .set entry to stdout or file
  encrypt  [-out FILE] PLAINFILE         encrypt a plaintext file to .set bytes
  search   [-archive A] [-case-sensitive] [-q] ROOT QUERY
  extract  -dest DIR [-entries a,b] [-decrypted] ARCHIVE
  tex      -out FILE ARCHIVE ENTRY       decode a .tex entry to PNG
```

### Examples

#### Decrypt a `.set` configuration file from an archive
```bash
# Print decrypted XML to stdout
./fanta-tennis-res-kit decrypt GameData.res LevelExp_Pet.set

# Save decrypted XML to file
./fanta-tennis-res-kit decrypt -out LevelExp_Pet.xml GameData.res LevelExp_Pet.set
```

#### Encrypt modified XML back to `.set` format
```bash
./fanta-tennis-res-kit encrypt -out LevelExp_Pet.set LevelExp_Pet.xml
```

#### Search for files across archives
```bash
# Search for pet-related files across all archives in the Res directory
./fanta-tennis-res-kit search /path/to/Res "Pet"

# Search inside a specific archive only
./fanta-tennis-res-kit search -archive Character.res /path/to/Res "Hair"
```

#### Extract archive entries
```bash
# Extract all files from an archive
./fanta-tennis-res-kit extract -dest ./extracted GameData.res

# Extract and automatically decrypt all .set files
./fanta-tennis-res-kit extract -dest ./extracted -decrypted GameData.res

# Extract specific entries
./fanta-tennis-res-kit extract -dest ./extracted -entries "LevelExp_Pet.set,Item.set" GameData.res
```

#### Decode `.tex` texture to PNG
```bash
./fanta-tennis-res-kit tex -out character_face.png Character.res Face_01.tex
```

### Exit Codes

| Code | Meaning | Description |
|:---:|:---|:---|
| `0` | Success | Operation completed successfully |
| `1` | Internal Error | Unexpected failure or uncaught error |
| `2` | Usage Error | Invalid command syntax, missing flag, or bad argument |
| `3` | Integrity Error | Corrupt data, bad padding, invalid cipher size, or invalid XML |
| `4` | I/O / Not Found | File not found or unreadable input archive / path |

---

## Architecture

The codebase strictly follows Clean Architecture:

- **[`domain/`](domain/)**: Pure domain rules, entity definitions, and sentinels (standard library only).
  - Pad math and validation (`setdoc.go`)
  - Texture descriptor and color modes (`texture.go`)
  - Archive entry kinds and matching (`archive.go`, `search.go`)
- **[`usecase/`](usecase/)**: Application workflows, interactor ports, and message boundaries.
  - Interactors: `ReadEntry`, `SaveEntry`, `Extract`, `PreviewTexture`, `PreviewImage`, `SearchAll`, `OpenWorkspace`, `ListEntries`.
  - Error classification logic (`errors.go`).
- **[`adapter/`](adapter/)**: Infrastructure and format adapters.
  - `reszip`: ZIP archive reader/writer with atomic replace and backup.
  - `aesecb`: Game AES-128-ECB cipher implementation.
  - `texdecoder`: DDS and DXT1/3/5 decoding to RGBA and PNG encoder.
  - `texenc`: DDS texture encoder (DXT1/3/5, RGB24/RGBA32, full mip chain).
  - `imgdecoder`: Extension dispatch for image leftovers (`.tex`/`.dds`/`.bmp`/`.tga` → PNG + metadata).
  - `disk`: Filesystem operations.
- **[`cmd/fanta-tennis-res-kit`](cmd/fanta-tennis-res-kit/)**: CLI composition root and entry point.

---

## Development & Testing

Run the test suite:
```bash
make test       # Run all unit tests
make vet        # Check for suspicious constructs
make fmt        # Check code formatting
```

To run integration tests against a read-only copy of client archives:
```bash
FANTA_RES_DIR=/path/to/read-only/Res make integration
```

---

## Support

If you find this project helpful and want to support its maintenance and further development:

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/S4V328HBWR)

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
