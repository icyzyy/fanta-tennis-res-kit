package reszip

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// Repo implements usecase.ArchiveRepo over standard ZIP files.
// Replace is atomic per archive: same-dir tmp → fsync → verify → rename,
// with an optional one-time .bak copy (default on). Readers and the writer
// of one archive are mutually excluded (per-archive RWMutex) and the
// writer's read handle is closed before the rename — Windows cannot
// replace a file while any handle to it is open.
type Repo struct {
	mapMu    sync.Mutex
	locks    map[string]*sync.RWMutex
	backup   bool
	backedUp map[string]bool
}

// New returns a repo with backup enabled.
func New() *Repo {
	return &Repo{
		locks:    map[string]*sync.RWMutex{},
		backedUp: map[string]bool{},
		backup:   true,
	}
}

// SetBackup toggles the one-time .bak copy before the first write per file.
func (r *Repo) SetBackup(enabled bool) {
	r.mapMu.Lock()
	r.backup = enabled
	r.mapMu.Unlock()
}

// lockFor returns the per-archive RWMutex: readers (List/Read) share it
// concurrently with each other but are excluded while Replace holds it —
// otherwise a reader's open handle can race the rename (a hard failure on
// Windows, where files are opened without FILE_SHARE_DELETE).
func (r *Repo) lockFor(archive string) *sync.RWMutex {
	r.mapMu.Lock()
	defer r.mapMu.Unlock()
	mu, ok := r.locks[archive]
	if !ok {
		mu = &sync.RWMutex{}
		r.locks[archive] = mu
	}
	return mu
}

func open(archive string) (*zip.ReadCloser, error) {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("open %s: %w", archive, err)
		}
		return nil, fmt.Errorf("%w: %s: %v", domain.ErrBadArchive, archive, err)
	}
	return zr, nil
}

// List reads the central directory only.
func (r *Repo) List(archive string) ([]domain.Entry, error) {
	mu := r.lockFor(archive)
	mu.RLock()
	defer mu.RUnlock()
	zr, err := open(archive)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	out := make([]domain.Entry, 0, len(zr.File))
	for _, f := range zr.File {
		out = append(out, domain.Entry{
			Name:             f.Name,
			Kind:             domain.KindOf(f.Name),
			CompressedSize:   int64(f.CompressedSize64),
			UncompressedSize: int64(f.UncompressedSize64),
			Method:           uint16(f.Method),
			ModTime:          f.Modified,
		})
	}
	return out, nil
}

// Read returns one entry's bytes.
func (r *Repo) Read(archive, entry string) ([]byte, error) {
	mu := r.lockFor(archive)
	mu.RLock()
	defer mu.RUnlock()
	zr, err := open(archive)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != entry {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("%w: %s/%s: %v", domain.ErrBadArchive, archive, entry, err)
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		if err != nil {
			return nil, fmt.Errorf("%w: %s/%s: %v", domain.ErrBadArchive, archive, entry, err)
		}
		return b, nil
	}
	return nil, fmt.Errorf("%w: %s in %s", domain.ErrNoSuchEntry, entry, archive)
}

// Replace rewrites one entry in place, atomically.
func (r *Repo) Replace(archive, entry string, data []byte) error {
	mu := r.lockFor(archive)
	mu.Lock()
	defer mu.Unlock()

	src, err := open(archive)
	if err != nil {
		return err
	}
	defer src.Close()
	found := false
	for _, f := range src.File {
		if f.Name == entry {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("%w: %s in %s", domain.ErrNoSuchEntry, entry, archive)
	}

	if err := r.backupOnce(archive); err != nil {
		return err
	}

	tmp := archive + ".tmp"
	_ = os.Remove(tmp) // stale tmp from an interrupted run
	if err := writeTmp(src, tmp, entry, data); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("%w: %v", domain.ErrReplaceFailed, err)
	}
	if err := verifyEntry(tmp, entry, data); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	// Windows: os.Rename (MoveFileEx REPLACE_EXISTING) fails with
	// ERROR_ACCESS_DENIED while any handle to the destination is open —
	// including our own read handle above, opened without
	// FILE_SHARE_DELETE. Close it explicitly before the rename; the
	// deferred Close then fails harmlessly (double Close returns an
	// error, it does not panic) and still covers every early return.
	_ = src.Close()
	if err := os.Rename(tmp, archive); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("%w: rename over %s: %v", domain.ErrReplaceFailed, archive, err)
	}
	return nil
}

// backupOnce copies the original to <archive>.bak before the first write of
// this session. A pre-existing .bak is preserved.
func (r *Repo) backupOnce(archive string) error {
	r.mapMu.Lock()
	enabled, done := r.backup, r.backedUp[archive]
	r.mapMu.Unlock()
	if !enabled || done {
		return nil
	}
	if _, err := os.Stat(archive + ".bak"); err == nil {
		r.mapMu.Lock()
		r.backedUp[archive] = true
		r.mapMu.Unlock()
		return nil
	}
	in, err := os.Open(archive)
	if err != nil {
		return fmt.Errorf("%w: backup open: %v", domain.ErrReplaceFailed, err)
	}
	defer in.Close()
	out, err := os.Create(archive + ".bak")
	if err != nil {
		return fmt.Errorf("%w: backup create: %v", domain.ErrReplaceFailed, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("%w: backup copy: %v", domain.ErrReplaceFailed, err)
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return fmt.Errorf("%w: backup sync: %v", domain.ErrReplaceFailed, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("%w: backup close: %v", domain.ErrReplaceFailed, err)
	}
	r.mapMu.Lock()
	r.backedUp[archive] = true
	r.mapMu.Unlock()
	return nil
}

// writeTmp copies the whole archive to tmp, substituting one entry.
func writeTmp(src *zip.ReadCloser, tmp, entry string, data []byte) (err error) {
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			f.Close()
		}
	}()
	zw := zip.NewWriter(f)
	for _, zf := range src.File {
		h := zf.FileHeader
		dst, err := zw.CreateHeader(&h)
		if err != nil {
			return err
		}
		if zf.Name == entry {
			if _, err := dst.Write(data); err != nil {
				return err
			}
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		_, err = io.Copy(dst, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return f.Close()
}

// verifyEntry reopens the tmp archive and checks the written entry.
func verifyEntry(path, entry string, want []byte) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("%w: verify reopen: %v", domain.ErrReplaceFailed, err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != entry {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("%w: verify open: %v", domain.ErrReplaceFailed, err)
		}
		got, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return fmt.Errorf("%w: verify read: %v", domain.ErrReplaceFailed, err)
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("%w: content mismatch for %s", domain.ErrReplaceFailed, entry)
		}
		return nil
	}
	return fmt.Errorf("%w: %s missing after write", domain.ErrReplaceFailed, entry)
}
