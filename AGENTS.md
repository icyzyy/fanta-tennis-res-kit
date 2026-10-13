# fanta-tennis-res-kit — agent instructions

Go library + CLI for the FantaTennis `Res/*.res` archives (browse, decrypt
`.set`, edit/repack, search, extract, decode/encode `.tex`, decode the
`.dds`/`.bmp`/`.tga` leftovers).

## Commands
- `make test` / `go test ./...` — unit tests (always run before committing)
- `make vet`, `make fmt` — must be clean
- `make integration` — needs `FANTA_RES_DIR` pointing at a READ-ONLY copy of
  the game's Res directory; never write into it

## Architecture (dependency rule: domain → usecase → adapter → driver)
- `domain/` — pure rules, stdlib only. Sentinels in errors.go, pad math in
  setdoc.go, entry kinds in archive.go, texture descriptors in texture.go,
  search matching in search.go.
- `usecase/` — interactors + ports (interfaces owned by consumers) +
  boundary types in messages.go + `Classify` for error codes.
  NO archive/zip, crypto/aes, image/png imports here.
- `adapter/` — reszip (ZIP repo, atomic replace), aesecb (`.set` cipher),
  texdecoder (`.tex` = header-inverted DDS), texenc (`.tex` encoder with full mip chain),
  imgdecoder (extension dispatch for image leftovers: `.tex` → texdecoder,
  `.dds` → header inverted back + texdecoder, `.bmp` → x/image/bmp,
  `.tga` → own decoder), disk (walk/stat/write).
- `cmd/fanta-tennis-res-kit` — CLI composition root; exit codes 0/1/2/3/4 =
  ok/internal/usage/integrity/IO.

## Naming
No `DTO` suffix — `usecase.Workspace`, not `usecase.WorkspaceDTO`. Error
types end in `Error`, sentinels start with `Err`.

## Format facts (reverse-engineered, verified on full corpus)
- `.set` = [1 byte pad][AES-128-ECB(zero-padded plaintext)];
  pad = (16 − len mod 16) mod 16 (0 when aligned); key = "TIMOTEI_ZION" +
  4 zero bytes; pad content ignored by the game (only length matters).
- `.tex` = standard DDS whose **128-byte header is byte-wise inverted**
  (game loader `FantaTennis.exe` `fcn.005DBC00`: `not` on 32 dwords only);
  the pixel **body is stored raw**. DXT1 auto-detected
  (4-color for predominantly opaque assets, order-based 1-bit alpha for cutouts);
  DXT3/DXT5 always 4-color. Decode mip0, tolerate trailing bytes.
- `.dds` / `.bmp` / `.tga` leftovers (16 / 2 / 4 corpus entries) are decoded
  by `adapter/imgdecoder` (usecase port `ImageDecoder`, interactor
  `NewPreviewImage` — no `KindOf` gate; extension dispatch lives there).
  A `.dds` is a **standard DDS with a plain, un-inverted header**: the
  `.dds` path inverts the first 128 bytes to synthesize a `.tex`-shaped
  buffer and reuses texdecoder. `.bmp` = classic BITMAPINFOHEADER via
  `golang.org/x/image/bmp`. `.tga` = types 1/2/3 + RLE 9/10/11, color-map
  entries 15/16/24/32-bit, depths 8/15/16/24/32; components stored BLUE
  first, bottom-left origin default / top when descriptor bit 0x20,
  X1R5G5B5 with attribute polarity 1 = transparent (cross-checked against
  Pillow; Pillow cannot read 32-bit color-mapped TGA, which is the corpus
  variant).

## Fixtures (committed, tests depend only on these)
- `testdata/LevelExp_Pet.set` (6129 B) + `testdata/LevelExp_Pet.xml` (6115 B)
- `testdata/tex/*.tex` + matching `*.png` oracles (7 files: DXT1×2, DXT3, DXT5,
  raw24, raw32×2)

## CLI (cmd/fanta-tennis-res-kit)
`fanta-tennis-res-kit decrypt|encrypt|search|extract|tex` — run
`go run ./cmd/fanta-tennis-res-kit help` for usage. Flags precede positional args.
Exit codes: 0 ok, 1 internal, 2 usage, 3 integrity (bad_pad/cipher_size/invalid_xml/...),
4 io/not-found. `cmdDecrypt` and `cmdTex` route through the ReadEntry/PreviewTexture
interactors; `encrypt` is a raw cipher transform.
