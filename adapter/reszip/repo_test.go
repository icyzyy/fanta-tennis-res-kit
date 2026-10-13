package reszip

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// buildZip creates an archive with the given entries (deflate unless store).
func buildZip(t *testing.T, path string, entries []struct {
	name  string
	data  string
	store bool
}) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.store {
			h.Method = zip.Store
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(e.data)); err != nil {
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

type zipEntry = struct {
	name  string
	data  string
	store bool
}

func readEntry(t *testing.T, archive, name string) []byte {
	t.Helper()
	r, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer rc.Close()
			b, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}
			return b
		}
	}
	t.Fatalf("entry %s not found in %s", name, archive)
	return nil
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.res")
	buildZip(t, a, []zipEntry{
		{"Gui_Login.set", "<a/>", false},
		{"UI01_Title.tex", "texdata", true},
		{"readme.txt", "hi", false},
	})
	repo := New()
	ents, err := repo.List(a)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ents) != 3 {
		t.Fatalf("List len = %d, want 3", len(ents))
	}
	if ents[0].Name != "Gui_Login.set" || ents[0].Kind != domain.KindSet {
		t.Errorf("ents[0] = %+v", ents[0])
	}
	if ents[1].Kind != domain.KindTex || ents[1].Method != zip.Store {
		t.Errorf("ents[1] = %+v", ents[1])
	}
	if ents[2].Kind != domain.KindOther || ents[2].UncompressedSize != 2 {
		t.Errorf("ents[2] = %+v", ents[2])
	}
}

func TestRead(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.res")
	buildZip(t, a, []zipEntry{{"x.set", "payload", false}})

	got, err := New().Read(a, "x.set")
	if err != nil || string(got) != "payload" {
		t.Fatalf("Read = %q, %v", got, err)
	}
	if _, err := New().Read(a, "missing.set"); !errors.Is(err, domain.ErrNoSuchEntry) {
		t.Errorf("missing entry err = %v, want ErrNoSuchEntry", err)
	}
	if _, err := New().Read(filepath.Join(dir, "nope.res"), "x.set"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing archive err = %v, want os.ErrNotExist", err)
	}
	corrupt := filepath.Join(dir, "bad.res")
	if err := os.WriteFile(corrupt, []byte("this is not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New().Read(corrupt, "x.set"); !errors.Is(err, domain.ErrBadArchive) {
		t.Errorf("corrupt archive err = %v, want ErrBadArchive", err)
	}
}

func TestReplaceAtomic(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.res")
	buildZip(t, a, []zipEntry{{"one.set", "first", false}, {"two.txt", "second", false}})
	before, _ := os.ReadFile(a)

	repo := New()
	if err := repo.Replace(a, "one.set", []byte("REWRITTEN")); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if got := readEntry(t, a, "one.set"); string(got) != "REWRITTEN" {
		t.Errorf("entry = %q", got)
	}
	if got := readEntry(t, a, "two.txt"); string(got) != "second" {
		t.Errorf("sibling entry clobbered: %q", got)
	}
	if _, err := os.Stat(a + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("tmp file left behind: %v", err)
	}
	// Backup of the ORIGINAL must exist (default on).
	bak, err := os.ReadFile(a + ".bak")
	if err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if !bytes.Equal(bak, before) {
		t.Error("backup does not match the original archive")
	}
	// Second replace must not refresh the backup.
	if err := repo.Replace(a, "one.set", []byte("TWICE")); err != nil {
		t.Fatal(err)
	}
	bak2, _ := os.ReadFile(a + ".bak")
	if !bytes.Equal(bak2, before) {
		t.Error("backup was overwritten on second write")
	}
}

func TestReplaceMissingEntryLeavesOriginalIntact(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.res")
	buildZip(t, a, []zipEntry{{"one.set", "first", false}})
	before, _ := os.ReadFile(a)

	repo := New()
	err := repo.Replace(a, "ghost.set", []byte("x"))
	if !errors.Is(err, domain.ErrNoSuchEntry) {
		t.Fatalf("err = %v, want ErrNoSuchEntry", err)
	}
	after, _ := os.ReadFile(a)
	if !bytes.Equal(before, after) {
		t.Error("original archive was modified despite failure")
	}
	if _, err := os.Stat(a + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("tmp left behind: %v", err)
	}
}

func TestReplaceBackupDisabled(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.res")
	buildZip(t, a, []zipEntry{{"one.set", "first", false}})

	repo := New()
	repo.SetBackup(false)
	if err := repo.Replace(a, "one.set", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a + ".bak"); !os.IsNotExist(err) {
		t.Errorf("backup created despite SetBackup(false): %v", err)
	}
}

func TestReplaceCorruptSource(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.res")
	if err := os.WriteFile(a, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := New().Replace(a, "x.set", []byte("y"))
	if !errors.Is(err, domain.ErrBadArchive) {
		t.Errorf("err = %v, want ErrBadArchive", err)
	}
}

func TestConcurrentReplaceDifferentArchives(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.res")
	b := filepath.Join(dir, "b.res")
	buildZip(t, a, []zipEntry{{"e.set", "A", false}})
	buildZip(t, b, []zipEntry{{"e.set", "B", false}})
	repo := New()
	var wg sync.WaitGroup
	for _, tc := range []struct {
		arch, val string
	}{{a, "A2"}, {b, "B2"}} {
		wg.Add(1)
		go func(arch, val string) {
			defer wg.Done()
			if err := repo.Replace(arch, "e.set", []byte(val)); err != nil {
				t.Errorf("Replace(%s): %v", arch, err)
			}
		}(tc.arch, tc.val)
	}
	wg.Wait()
	if got := readEntry(t, a, "e.set"); string(got) != "A2" {
		t.Errorf("a.res = %q", got)
	}
	if got := readEntry(t, b, "e.set"); string(got) != "B2" {
		t.Errorf("b.res = %q", got)
	}
}
