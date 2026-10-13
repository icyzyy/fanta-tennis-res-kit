package usecase

// ProgressEvent reports coarse progress of a long operation.
// Err (optional) marks a skipped/failed unit without aborting the run.
type ProgressEvent struct {
	Op    string
	Done  int
	Total int
	Path  string
	Err   string
}

// ProgressPort receives progress events. Implementations must be safe for
// concurrent use (interactors may report from worker goroutines).
type ProgressPort interface {
	Report(ev ProgressEvent)
}

// ProgressFunc adapts a function to ProgressPort.
type ProgressFunc func(ProgressEvent)

func (f ProgressFunc) Report(ev ProgressEvent) { f(ev) }
