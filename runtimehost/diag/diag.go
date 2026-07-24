// Package diag provides a tiny, dependency-free diagnostic logging sink that
// the IKEv2 implementation (and any other internal package) can call without
// creating an import cycle on the top-level runtimehost package.
//
// Wiring is performed once at startup: runtimehost.SetLogger stores a LogSink
// in this package's package-level sink variable. Implementation packages
// (ikev2, runtimehost) import this package to emit messages. Only this
// package knows about runtimehost's LogSink type — there is no reverse
// import.
package diag

import "sync"

// LogSink receives text messages from IKEv2 / runtimehost internals.
// Implementations should be fast and non-blocking; writers may be any
// goroutine.
type LogSink interface {
	Write(level, msg string)
}

type noopSink struct{}

func (noopSink) Write(level, msg string) {}

var (
	mu   sync.RWMutex
	sink LogSink = noopSink{}
)

// SetLogger installs a logger sink. Pass nil to restore the no-op default.
// Safe to call once at startup.
func SetLogger(s LogSink) {
	mu.Lock()
	defer mu.Unlock()
	if l := s; l == nil {
		_ = l
		sink = noopSink{}
		return
	}
	sink = s
}

// Log emits a message to the configured sink. No-op when the default sink is
// installed.
func Log(level, msg string) {
	mu.RLock()
	s := sink
	mu.RUnlock()
	s.Write(level, msg)
}