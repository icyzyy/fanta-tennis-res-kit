package usecase

import (
	"bytes"
	"image"
	"sync"
	"time"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

var (
	_ ArchiveRepo    = (*fakeRepo)(nil)
	_ SetCipher      = (*stubCipher)(nil)
	_ TextureDecoder = (*stubTex)(nil)
	_ TextureEncoder = (*stubTexEnc)(nil)
	_ ImageDecoder   = (*stubImg)(nil)
	_ FileSystem     = (*fakeFS)(nil)
	_ ProgressPort   = (*recProgress)(nil)
	_ ProgressPort   = ProgressFunc(nil)
)

// fakeRepo is a programmable in-memory ArchiveRepo. An explicit readFn
// script takes precedence; otherwise Read serves bytes written by Replace,
// which lets interactor round-trip verifies (SaveEntry) work like real I/O.
type fakeRepo struct {
	mu        sync.Mutex
	listFn    func(archive string) ([]domain.Entry, error)
	readFn    func(archive, entry string) ([]byte, error)
	replaceFn func(archive, entry string, data []byte) error
	backup    bool
	replaced  []string // "archive/entry"
	store     map[string][]byte
}

func (f *fakeRepo) List(archive string) ([]domain.Entry, error) {
	if f.listFn != nil {
		return f.listFn(archive)
	}
	return nil, nil
}

func (f *fakeRepo) Read(archive, entry string) ([]byte, error) {
	if f.readFn != nil {
		return f.readFn(archive, entry)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if b, ok := f.store[archive+"/"+entry]; ok {
		return append([]byte(nil), b...), nil
	}
	return nil, domain.ErrNoSuchEntry
}

func (f *fakeRepo) Replace(archive, entry string, data []byte) error {
	f.mu.Lock()
	f.replaced = append(f.replaced, archive+"/"+entry)
	f.mu.Unlock()
	if f.replaceFn != nil {
		if err := f.replaceFn(archive, entry, data); err != nil {
			return err
		}
	}
	f.mu.Lock()
	if f.store == nil {
		f.store = map[string][]byte{}
	}
	f.store[archive+"/"+entry] = append([]byte(nil), data...)
	f.mu.Unlock()
	return nil
}

func (f *fakeRepo) SetBackup(enabled bool) {
	f.mu.Lock()
	f.backup = enabled
	f.mu.Unlock()
}

func (f *fakeRepo) replacedLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.replaced...)
}

// stubCipher scripts encrypt/decrypt; defaults round-trip through "enc:".
type stubCipher struct {
	enc func([]byte) ([]byte, error)
	dec func([]byte) ([]byte, error)
}

func (c *stubCipher) Encrypt(p []byte) ([]byte, error) {
	if c.enc != nil {
		return c.enc(p)
	}
	return append([]byte("enc:"), p...), nil
}

func (c *stubCipher) Decrypt(b []byte) ([]byte, error) {
	if c.dec != nil {
		return c.dec(b)
	}
	return bytes.TrimPrefix(b, []byte("enc:")), nil
}

// stubTex returns a scripted texture result.
type stubTex struct {
	meta *domain.Texture
	png  []byte
	err  error
}

func (s *stubTex) Decode(tex []byte) (*domain.Texture, []byte, error) {
	if s.err != nil {
		return nil, nil, s.err
	}
	return s.meta, s.png, nil
}

// stubTexEnc returns a scripted texture encoding result.
type stubTexEnc struct {
	tex []byte
	err error
}

func (s *stubTexEnc) Encode(src image.Image, format domain.TextureFormat) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.tex, nil
}

// stubImg returns a scripted image decode result and records the dispatch
// arguments it received.
type stubImg struct {
	meta    *domain.Texture
	png     []byte
	err     error
	gotName string
	gotData []byte
}

func (s *stubImg) Decode(name string, data []byte) (*domain.Texture, []byte, error) {
	s.gotName = name
	s.gotData = data
	if s.err != nil {
		return nil, nil, s.err
	}
	return s.meta, s.png, nil
}

// fakeFS scripts file-system responses.
type fakeFS struct {
	mu        sync.Mutex
	res       map[string][]string
	resErr    map[string]error
	statFn    func(path string) (int64, time.Time, error)
	written   map[string][]byte
	statCalls []string
}

func newFakeFS() *fakeFS {
	return &fakeFS{
		res:     map[string][]string{},
		resErr:  map[string]error{},
		written: map[string][]byte{},
	}
}

func (f *fakeFS) WalkRes(root string) ([]string, error) {
	if err, ok := f.resErr[root]; ok {
		return nil, err
	}
	return append([]string(nil), f.res[root]...), nil
}

func (f *fakeFS) Stat(path string) (int64, time.Time, error) {
	f.mu.Lock()
	f.statCalls = append(f.statCalls, path)
	f.mu.Unlock()
	if f.statFn != nil {
		return f.statFn(path)
	}
	return 100, time.Unix(1000, 0), nil
}

func (f *fakeFS) WriteFile(path string, data []byte) error {
	f.mu.Lock()
	f.written[path] = append([]byte(nil), data...)
	f.mu.Unlock()
	return nil
}

func (f *fakeFS) statCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.statCalls)
}

// recProgress records progress events; safe for concurrent use.
type recProgress struct {
	mu  sync.Mutex
	evs []ProgressEvent
}

func (r *recProgress) Report(ev ProgressEvent) {
	if r == nil {
		return // tolerate a typed-nil ProgressPort (collect(sa, req, nil))
	}
	r.mu.Lock()
	r.evs = append(r.evs, ev)
	r.mu.Unlock()
}

func (r *recProgress) snapshot() []ProgressEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]ProgressEvent(nil), r.evs...)
}
