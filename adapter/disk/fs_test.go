package disk

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWalkRes(t *testing.T) {
	root := t.TempDir()
	mk := func(rel string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("a.res")
	mk("sub/b.RES")
	mk("sub/deep/c.res")
	mk("sub/notes.txt")
	mk("dir.res/inside.txt") // a DIRECTORY named *.res must be skipped

	got, err := New().WalkRes(root)
	if err != nil {
		t.Fatalf("WalkRes: %v", err)
	}
	want := []string{
		filepath.Join(root, "a.res"),
		filepath.Join(root, "sub", "b.RES"),
		filepath.Join(root, "sub", "deep", "c.res"),
	}
	if len(got) != len(want) {
		t.Fatalf("WalkRes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("WalkRes[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestWalkResMissingRoot(t *testing.T) {
	if _, err := New().WalkRes(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("WalkRes on missing root: want error, got nil")
	}
}

func TestStat(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	size, mt, err := New().Stat(p)
	if err != nil || size != 3 {
		t.Fatalf("Stat = %d, %v; want 3, nil", size, err)
	}
	if mt.After(time.Now().Add(time.Minute)) {
		t.Errorf("implausible mod time %v", mt)
	}
}

func TestWriteFileCreatesParents(t *testing.T) {
	p := filepath.Join(t.TempDir(), "deep", "nested", "out.xml")
	if err := New().WriteFile(p, []byte("<a/>")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := os.ReadFile(p)
	if err != nil || string(got) != "<a/>" {
		t.Fatalf("read back %q, %v", got, err)
	}
}
