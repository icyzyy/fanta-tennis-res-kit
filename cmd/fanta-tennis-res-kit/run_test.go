package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

type archiveEntry struct {
	name string
	data []byte
}

func buildArchive(t *testing.T, path string, entries []archiveEntry) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", rel))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRunUsage(t *testing.T) {
	cases := []struct {
		args []string
		code int
	}{
		{nil, 2},
		{[]string{"bogus"}, 2},
		{[]string{"help"}, 0},
		{[]string{"decrypt"}, 2},               // missing positional args
		{[]string{"decrypt", "-out"}, 2},       // flag missing its value
		{[]string{"decrypt", "-h"}, 0},         // per-command help
		{[]string{"tex", "a.res", "x.tex"}, 2}, // -out required
		{[]string{"extract", "a.res"}, 2},      // -dest required
	}
	for _, c := range cases {
		var stdout, stderr bytes.Buffer
		if got := run(c.args, &stdout, &stderr); got != c.code {
			t.Errorf("run(%v) = %d, want %d (stderr: %s)", c.args, got, c.code, stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	run([]string{"help"}, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "usage: fanta-tennis-res-kit") {
		t.Errorf("help output = %q", stdout.String())
	}
}

func TestExitCodeTable(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{domain.ErrBadQuery, 2},
		{domain.ErrBadDestination, 2},
		{domain.ErrBadPad, 3},
		{domain.ErrNotSetDoc, 3},
		{domain.ErrBadCipherSize, 3},
		{domain.ErrInvalidXML, 3},
		{domain.ErrNotTexture, 3},
		{domain.ErrNoSuchEntry, 4},
		{os.ErrPermission, 4},
		{errors.New("mystery"), 1},
	}
	for _, c := range cases {
		if got := exitCode(c.err); got != c.code {
			t.Errorf("exitCode(%v) = %d, want %d", c.err, got, c.code)
		}
	}
}

func TestRunEncryptDecryptRoundTrip(t *testing.T) {
	dir := t.TempDir()
	plain := fixture(t, "LevelExp_Pet.xml")
	plainPath := filepath.Join(dir, "in.xml")
	if err := os.WriteFile(plainPath, plain, 0o644); err != nil {
		t.Fatal(err)
	}
	setPath := filepath.Join(dir, "out.set")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"encrypt", "-out", setPath, plainPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("encrypt = %d: %s", code, stderr.String())
	}
	setBytes, err := os.ReadFile(setPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(setBytes, fixture(t, "LevelExp_Pet.set")) {
		t.Error("encrypt did not reproduce the golden .set bytes")
	}

	archive := filepath.Join(dir, "Script.res")
	buildArchive(t, archive, []archiveEntry{{"LevelExp_Pet.set", setBytes}})

	gotPath := filepath.Join(dir, "out.xml")
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"decrypt", "-out", gotPath, archive, "LevelExp_Pet.set"}, &stdout, &stderr); code != 0 {
		t.Fatalf("decrypt = %d: %s", code, stderr.String())
	}
	got, err := os.ReadFile(gotPath)
	if err != nil || !bytes.Equal(got, plain) {
		t.Errorf("CLI round-trip mismatch: %v", err)
	}

	// decrypt to stdout by default
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"decrypt", archive, "LevelExp_Pet.set"}, &stdout, &stderr); code != 0 {
		t.Fatalf("decrypt stdout = %d: %s", code, stderr.String())
	}
	if !bytes.Equal(stdout.Bytes(), plain) {
		t.Error("stdout decrypt mismatch")
	}
}

func TestRunDecryptExitCodes(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.res")
	buildArchive(t, archive, []archiveEntry{
		{"bad.set", []byte{0, 1, 2, 3}}, // ciphertext not a multiple of 16
		{"x.txt", []byte("hello")},
	})
	cases := []struct {
		name string
		args []string
		code int
	}{
		{"missing archive", []string{"decrypt", filepath.Join(dir, "nope.res"), "a.set"}, 4},
		{"missing entry", []string{"decrypt", archive, "ghost.set"}, 4},
		{"corrupt set", []string{"decrypt", archive, "bad.set"}, 3},
		{"non-set entry", []string{"decrypt", archive, "x.txt"}, 3},
		{"missing encrypt input", []string{"encrypt", filepath.Join(dir, "no.xml")}, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(c.args, &stdout, &stderr); got != c.code {
				t.Errorf("run = %d, want %d (stderr: %s)", got, c.code, stderr.String())
			}
		})
	}
}

func TestRunSearch(t *testing.T) {
	dir := t.TempDir()
	plain := []byte("<?xml version=\"1.0\"?>\n<Root>\n\t<Val>AlphaValue one</Val>\n\t<Other>nothing</Other>\n</Root>\n")
	plainPath := filepath.Join(dir, "doc.xml")
	if err := os.WriteFile(plainPath, plain, 0o644); err != nil {
		t.Fatal(err)
	}
	setPath := filepath.Join(dir, "doc.set")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"encrypt", "-out", setPath, plainPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("encrypt = %d: %s", code, stderr.String())
	}
	setBytes, _ := os.ReadFile(setPath)
	buildArchive(t, filepath.Join(dir, "Gui3.res"), []archiveEntry{{"Gui_Login.set", setBytes}})

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"search", "-q", dir, "alphavalue"}, &stdout, &stderr); code != 0 {
		t.Fatalf("search = %d: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Gui_Login.set") || !strings.Contains(out, " : 3\t") || !strings.Contains(out, "AlphaValue one") {
		t.Errorf("search output = %q", out)
	}
	// case-sensitive misses the lowercase query
	stdout.Reset()
	if code := run([]string{"search", "-q", "-case-sensitive", dir, "alphavalue"}, &stdout, &stderr); code != 0 || stdout.Len() != 0 {
		t.Errorf("case-sensitive search: code=%d out=%q", code, stdout.String())
	}
}

func TestRunExtract(t *testing.T) {
	dir := t.TempDir()
	plain := fixture(t, "LevelExp_Pet.xml")
	plainPath := filepath.Join(dir, "in.xml")
	if err := os.WriteFile(plainPath, plain, 0o644); err != nil {
		t.Fatal(err)
	}
	setPath := filepath.Join(dir, "in.set")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"encrypt", "-out", setPath, plainPath}, &stdout, &stderr); code != 0 {
		t.Fatal("encrypt failed: " + stderr.String())
	}
	setBytes, _ := os.ReadFile(setPath)
	archive := filepath.Join(dir, "Ini3.res")
	buildArchive(t, archive, []archiveEntry{
		{"LevelExp_Pet.set", setBytes},
		{"Sub/notes.txt", []byte("hi")},
	})

	// Whole archive, raw (encrypted .set bytes stay encrypted).
	rawDir := filepath.Join(dir, "raw")
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"extract", "-dest", rawDir, archive}, &stdout, &stderr); code != 0 {
		t.Fatalf("extract = %d: %s", code, stderr.String())
	}
	got, err := os.ReadFile(filepath.Join(rawDir, "LevelExp_Pet.set"))
	if err != nil || !bytes.Equal(got, setBytes) {
		t.Errorf("raw extract: %v", err)
	}
	if _, err := os.Stat(filepath.Join(rawDir, "Sub", "notes.txt")); err != nil {
		t.Errorf("subdir entry not extracted: %v", err)
	}

	// Decrypted subset.
	decDir := filepath.Join(dir, "dec")
	stdout.Reset()
	if code := run([]string{"extract", "-dest", decDir, "-decrypted", "-entries", "LevelExp_Pet.set", archive}, &stdout, &stderr); code != 0 {
		t.Fatalf("extract decrypted = %d: %s", code, stderr.String())
	}
	got, err = os.ReadFile(filepath.Join(decDir, "LevelExp_Pet.set"))
	if err != nil || !bytes.Equal(got, plain) {
		t.Errorf("decrypted extract mismatch: %v", err)
	}
	if _, err := os.Stat(filepath.Join(decDir, "Sub")); !os.IsNotExist(err) {
		t.Error("subset extract wrote extra files")
	}

	// Missing requested entry → 4.
	stdout.Reset()
	if code := run([]string{"extract", "-dest", decDir, "-entries", "ghost.set", archive}, &stdout, &stderr); code != 4 {
		t.Errorf("missing entry code = %d, want 4", code)
	}
}

func TestRunTex(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "EftB.res")
	buildArchive(t, archive, []archiveEntry{
		{"A_fire_solt_Sphere_D.tex", fixture(t, "tex/A_fire_solt_Sphere_D.tex")},
	})
	outPNG := filepath.Join(dir, "out.png")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"tex", "-out", outPNG, archive, "A_fire_solt_Sphere_D.tex"}, &stdout, &stderr); code != 0 {
		t.Fatalf("tex = %d: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "4x4 DXT1") {
		t.Errorf("meta line = %q", stderr.String())
	}
	got, err := os.ReadFile(outPNG)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatalf("output is not a PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 4 || b.Dy() != 4 {
		t.Errorf("png dims %dx%d, want 4x4", b.Dx(), b.Dy())
	}
}
