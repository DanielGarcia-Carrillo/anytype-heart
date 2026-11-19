package runstore

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	anystore "github.com/anyproto/any-store/v2"
)

// dbHandle is the one any-store handle of a run db in this process.
//
// any-store v2 refuses a second open of a file in the process that already
// holds it open (btree: database already open in this process), so every
// Store of one run db shares one handle: a second holder of a live run
// (Invariant 3), and a status poll reading beside the live writer (review
// Class E). The handle is refcounted; the last holder closes it.
//
// A handle remembers how it was opened. A writable handle (Create, Open,
// OpenExclusive) has the durability block of storeConfig; a status reader
// joins it as it is. A handle a status reader opened has none of that — no
// sentinel, no auto-flush — so a writable open never joins it: it waits
// until the readers have closed it, then opens the db itself. Readers are
// short-lived, one status poll each.
type dbHandle struct {
	path     string
	writable bool

	// opened is closed once the open has finished; db or err is set then.
	opened chan struct{}
	db     anystore.DB
	err    error

	// refs and closing are guarded by handlesMu.
	refs    int
	closing bool
	// closed is closed once the handle is gone from the registry: its db is
	// closed, or its open failed.
	closed chan struct{}
}

var (
	handlesMu sync.Mutex
	handles   = map[string]*dbHandle{}
)

// acquireDb returns the process's handle of the run db at path, opening it
// with config when no handle exists. Every successful call is paired with
// one release.
func acquireDb(ctx context.Context, path string, writable bool, config *anystore.Config) (*dbHandle, error) {
	key := filepath.Clean(path)
	for {
		handlesMu.Lock()
		h := handles[key]
		if h == nil {
			h = &dbHandle{
				path:     key,
				writable: writable,
				opened:   make(chan struct{}),
				refs:     1,
				closed:   make(chan struct{}),
			}
			handles[key] = h
			handlesMu.Unlock()
			return h.open(ctx, config)
		}
		if h.closing || (writable && !h.writable) {
			// The last holder is closing the db, or only status readers hold
			// it: open after them.
			handlesMu.Unlock()
			select {
			case <-h.closed:
				continue
			case <-ctx.Done():
				return nil, fmt.Errorf("wait for the run db to be released: %w", ctx.Err())
			}
		}
		h.refs++
		handlesMu.Unlock()
		<-h.opened
		if h.err != nil {
			// The failed handle left the registry with its opener; there is
			// nothing to release.
			return nil, h.err
		}
		return h, nil
	}
}

func (h *dbHandle) open(ctx context.Context, config *anystore.Config) (*dbHandle, error) {
	defer close(h.opened)
	h.db, h.err = anystore.Open(ctx, h.path, config)
	if h.err != nil {
		handlesMu.Lock()
		delete(handles, h.path)
		handlesMu.Unlock()
		close(h.closed)
		return nil, h.err
	}
	return h, nil
}

// release drops one hold; the last one closes the db. The handle stays in
// the registry until the db is closed, so a new open of the file waits for
// the close instead of failing on the still-open file.
func (h *dbHandle) release() error {
	handlesMu.Lock()
	h.refs--
	if h.refs > 0 {
		handlesMu.Unlock()
		return nil
	}
	h.closing = true
	handlesMu.Unlock()

	err := h.db.Close()

	handlesMu.Lock()
	delete(handles, h.path)
	handlesMu.Unlock()
	close(h.closed)
	return err
}
