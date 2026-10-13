//go:build integration

package main

import (
	"context"
	"os"
	"testing"

	"github.com/icyzyy/fanta-tennis-res-kit/adapter/aesecb"
	"github.com/icyzyy/fanta-tennis-res-kit/adapter/disk"
	"github.com/icyzyy/fanta-tennis-res-kit/adapter/reszip"
	"github.com/icyzyy/fanta-tennis-res-kit/adapter/texdecoder"
	"github.com/icyzyy/fanta-tennis-res-kit/domain"
	"github.com/icyzyy/fanta-tennis-res-kit/usecase"
)

// TestRealWorkspaceReadOnly exercises the interactors against a real copy of
// the game's Res directory (FANTA_RES_DIR). READ-ONLY by design: the test
// only lists, reads and decodes — it never calls Replace/WriteFile, so it is
// safe to point at a shared, read-only Res copy.
func TestRealWorkspaceReadOnly(t *testing.T) {
	root := os.Getenv("FANTA_RES_DIR")
	if root == "" {
		t.Skip("FANTA_RES_DIR not set")
	}
	repo, cipher, fsys, tex := reszip.New(), aesecb.New(), disk.New(), texdecoder.New()

	ws, err := usecase.NewOpenWorkspace(fsys, nil).Do(context.Background(), root)
	if err != nil {
		t.Fatalf("open workspace: %v", err)
	}
	if len(ws.Archives) == 0 {
		t.Fatal("no archives found")
	}
	t.Logf("archives: %d", len(ws.Archives))

	lister := usecase.NewListEntries(repo)
	decrypter := usecase.NewReadEntry(repo, cipher, tex)
	preview := usecase.NewPreviewTexture(repo, tex)

	var checkedSet, checkedTex bool
	for _, a := range ws.Archives {
		ents, err := lister.Do(context.Background(), a.Path)
		if err != nil {
			t.Errorf("list %s: %v", a.Path, err)
			continue
		}
		for _, e := range ents {
			switch e.Kind {
			case domain.KindSet:
				c, err := decrypter.Do(context.Background(), a.Path, e.Name)
				if err != nil {
					t.Errorf("decrypt %s/%s: %v", a.Path, e.Name, err)
				} else if c.Kind != usecase.ContentXML || len(c.Text) == 0 {
					t.Errorf("bad content for %s/%s", a.Path, e.Name)
				}
				checkedSet = true
			case domain.KindTex:
				meta, pngBytes, err := preview.Do(context.Background(), a.Path, e.Name)
				if err != nil {
					t.Errorf("tex %s/%s: %v", a.Path, e.Name, err)
				} else if meta.Width == 0 || len(pngBytes) == 0 {
					t.Errorf("empty texture %s/%s", a.Path, e.Name)
				}
				checkedTex = true
			}
			if checkedSet && checkedTex {
				return
			}
		}
	}
	t.Fatalf("did not find both a .set and a .tex to check (set=%v tex=%v)", checkedSet, checkedTex)
}
