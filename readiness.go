package main

import "sync"

// Readiness tracks whether each side has finished receiving “web data” and “asset data”
type Readiness struct {
	mu              sync.RWMutex
	copyWebReady    bool
	pasteWebReady   bool
	copyAssetReady  bool
	pasteAssetReady bool
}

func NewReadiness() *Readiness {
	return &Readiness{}
}

func (r *Readiness) SetWebReady(copySide bool, pasteSide bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if copySide {
		r.copyWebReady = true
	}
	if pasteSide {
		r.pasteWebReady = true
	}
}

func (r *Readiness) SetAssetReady(copySide bool, pasteSide bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if copySide {
		r.copyAssetReady = true
	}
	if pasteSide {
		r.pasteAssetReady = true
	}
}

// AllReady returns true if the copy & paste sides are fully ready
func (r *Readiness) AllReady() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.copyWebReady && r.pasteWebReady && r.copyAssetReady && r.pasteAssetReady
}
