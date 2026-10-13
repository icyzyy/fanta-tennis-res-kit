package usecase

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestOpenWorkspace(t *testing.T) {
	fs := newFakeFS()
	fs.res["/game/Res"] = []string{"/game/Res/Gui3.res", "/game/Res/Item.res"}
	fs.statFn = func(path string) (int64, time.Time, error) {
		return 42, time.Unix(1700000000, 0), nil
	}
	prog := &recProgress{}

	ws, err := NewOpenWorkspace(fs, prog).Do(context.Background(), "/game/Res")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if ws.Root != "/game/Res" || len(ws.Archives) != 2 {
		t.Fatalf("ws = %+v", ws)
	}
	if ws.Archives[0].Path != "/game/Res/Gui3.res" || ws.Archives[0].Size != 42 {
		t.Errorf("Archives[0] = %+v", ws.Archives[0])
	}
	if ws.Archives[0].ModTime != 1700000000 {
		t.Errorf("ModTime = %d, want unix 1700000000", ws.Archives[0].ModTime)
	}
	evs := prog.snapshot()
	if len(evs) != 2 {
		t.Fatalf("progress events = %d, want 2", len(evs))
	}
	if evs[0].Op != "open_workspace" || evs[0].Total != 2 || evs[1].Done != 2 {
		t.Errorf("events = %+v", evs)
	}
}

func TestOpenWorkspaceWalkError(t *testing.T) {
	fs := newFakeFS()
	fs.resErr["/bad"] = errors.New("permission denied")
	_, err := NewOpenWorkspace(fs, &recProgress{}).Do(context.Background(), "/bad")
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if got := err.Error(); len(got) < 14 || got[:14] != "open_workspace" {
		t.Errorf("error not wrapped: %v", err)
	}
}

func TestOpenWorkspaceStatFailureSkipped(t *testing.T) {
	fs := newFakeFS()
	fs.res["/r"] = []string{"/r/gone.res"}
	fs.statFn = func(path string) (int64, time.Time, error) {
		return 0, time.Time{}, errors.New("vanished")
	}
	prog := &recProgress{}
	ws, err := NewOpenWorkspace(fs, prog).Do(context.Background(), "/r")
	if err != nil {
		t.Fatalf("stat failure must not abort: %v", err)
	}
	if len(ws.Archives) != 0 {
		t.Errorf("unstatable archive should be skipped: %+v", ws.Archives)
	}
	evs := prog.snapshot()
	if len(evs) != 1 || evs[0].Err == "" {
		t.Errorf("skip not reported: %+v", evs)
	}
}

func TestOpenWorkspaceCancelled(t *testing.T) {
	fs := newFakeFS()
	fs.res["/r"] = []string{"/r/a.res"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewOpenWorkspace(fs, &recProgress{}).Do(ctx, "/r")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
