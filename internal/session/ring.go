package session

// ring keeps the most recent bytes written to it, up to a fixed capacity.
// It is not safe for concurrent use; Session guards it.
type ring struct {
	buf  []byte
	size int
	full bool
	pos  int
	// total counts every byte ever written, so a position in the stream
	// stays meaningful after old bytes have been overwritten.
	total uint64
}

func newRing(capacity int) *ring {
	return &ring{buf: make([]byte, capacity), size: capacity}
}

func (r *ring) write(p []byte) {
	r.total += uint64(len(p))
	if len(p) >= r.size {
		copy(r.buf, p[len(p)-r.size:])
		r.pos, r.full = 0, true
		return
	}
	n := copy(r.buf[r.pos:], p)
	if n < len(p) {
		copy(r.buf, p[n:])
		r.full = true
	}
	r.pos = (r.pos + len(p)) % r.size
	if r.pos == 0 && len(p) > 0 {
		r.full = true
	}
}

// snapshot returns a copy of the retained bytes, oldest first.
func (r *ring) snapshot() []byte {
	if !r.full {
		return append([]byte(nil), r.buf[:r.pos]...)
	}
	out := make([]byte, 0, r.size)
	out = append(out, r.buf[r.pos:]...)
	return append(out, r.buf[:r.pos]...)
}

// since returns the bytes written from stream position from onwards, and ok
// when the ring still holds all of them. A position beyond what was written
// (from a different run) or already overwritten is not ok.
func (r *ring) since(from uint64) (data []byte, ok bool) {
	snap := r.snapshot()
	if from > r.total || r.total-from > uint64(len(snap)) {
		return nil, false
	}
	return snap[len(snap)-int(r.total-from):], true
}

// start is the stream position of the first retained byte.
func (r *ring) start() uint64 { return r.total - uint64(len(r.snapshot())) }
