package usecase

import (
	"context"
	"fmt"
	"sort"
)

// OpenWorkspace discovers every *.res archive under a root directory.
type OpenWorkspace struct {
	fs       FileSystem
	progress ProgressPort
}

// NewOpenWorkspace wires the interactor.
func NewOpenWorkspace(fs FileSystem, progress ProgressPort) *OpenWorkspace {
	return &OpenWorkspace{fs: fs, progress: progress}
}

// Do walks root and returns the archive list, sorted by path. Unreadable
// individual archives are skipped and surfaced through progress events;
// only a failed walk aborts.
func (x *OpenWorkspace) Do(ctx context.Context, root string) (*Workspace, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("open_workspace: %w", err)
	}
	paths, err := x.fs.WalkRes(root)
	if err != nil {
		return nil, fmt.Errorf("open_workspace: %w", err)
	}
	ws := &Workspace{Root: root, Archives: make([]ArchiveInfo, 0, len(paths))}
	for i, p := range paths {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("open_workspace: %w", err)
		}
		size, mt, err := x.fs.Stat(p)
		ev := ProgressEvent{Op: "open_workspace", Done: i + 1, Total: len(paths), Path: p}
		if err != nil {
			ev.Err = err.Error()
		} else {
			ws.Archives = append(ws.Archives, ArchiveInfo{
				Path:    p,
				Size:    size,
				ModTime: mt.Unix(),
			})
		}
		if x.progress != nil {
			x.progress.Report(ev)
		}
	}
	sort.Slice(ws.Archives, func(i, j int) bool {
		return ws.Archives[i].Path < ws.Archives[j].Path
	})
	return ws, nil
}
