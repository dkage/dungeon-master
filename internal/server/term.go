package server

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// Terminal websocket protocol.
//
//	server → client: first a text frame {"type":"attach","start":N,"resume":B}
//	(N is the stream position of the first byte that follows; B says the
//	replay continues from the position the page asked for with ?from=), then
//	binary frames of terminal output (the first one is the replay); one final
//	text frame {"type":"exit","code":N} when the process ends.
//	client → server: binary frames are keystrokes; text frames are JSON
//	control messages, currently {"type":"resize","cols":N,"rows":N}.
func (s *Server) handleTerm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, ok := s.cfg.Sessions.Get(id)
	if !ok {
		if rec, known := s.cfg.History.Get(id); known && rec.Lost {
			httpErr(w, http.StatusGone, "this session ended when Dungeon Master was last stopped")
			return
		}
		httpErr(w, http.StatusNotFound, "no such session")
		return
	}
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return // Accept already replied.
	}
	defer c.CloseNow()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	started := time.Now()
	var lastHeard atomic.Int64 // unix nanoseconds of the last sign of life from the peer
	lastHeard.Store(started.UnixNano())
	cause := make(chan string, 1) // the first cause wins
	note := func(why string) {
		select {
		case cause <- why:
		default:
		}
	}
	end := func(why string) {
		note(why)
		cancel()
	}
	// One line per connection: ids and durations only, never keystrokes,
	// output, tokens or issue text.
	defer func() {
		why := "unknown"
		select {
		case why = <-cause:
		default:
		}
		s.cfg.Log.Info("terminal connection closed",
			"session", id, "cause", why,
			"lasted", time.Since(started).Round(time.Millisecond),
			"silent", time.Since(time.Unix(0, lastHeard.Load())).Round(time.Millisecond))
	}()

	// A page that already holds the stream up to some position says so and is
	// replayed only what it missed; anything else gets the whole scrollback.
	from := uint64(math.MaxUint64)
	if q := r.URL.Query().Get("from"); q != "" {
		if n, err := strconv.ParseUint(q, 10, 64); err == nil {
			from = n
		}
	}
	replay, start, resumed, sub := sess.AttachFrom(from)
	defer sess.Detach(sub)
	attach, _ := json.Marshal(map[string]any{"type": "attach", "start": start, "resume": resumed})
	if err := c.Write(ctx, websocket.MessageText, attach); err != nil {
		end("write error")
		return
	}

	// Reader: keystrokes and control messages.
	go func() {
		for {
			typ, data, err := c.Read(ctx)
			if err != nil {
				switch {
				case ctx.Err() != nil:
					end("server shut down or connection ended")
				case websocket.CloseStatus(err) != -1:
					end("client closed")
				default:
					end("read error")
				}
				return
			}
			lastHeard.Store(time.Now().UnixNano())
			switch typ {
			case websocket.MessageBinary:
				if _, err := sess.Write(data); err != nil {
					end("session exited")
					return
				}
			case websocket.MessageText:
				var msg struct {
					Type       string `json:"type"`
					Cols, Rows int
				}
				if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" {
					_ = sess.Resize(msg.Cols, msg.Rows)
				}
			}
		}
	}()

	// Keepalive: a ping keeps intermediaries from dropping an idle
	// connection, and a peer that stops answering is closed here.
	go func() {
		t := time.NewTicker(s.cfg.TermPing)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
			pctx, pcancel := context.WithTimeout(ctx, s.cfg.TermPingTimeout)
			err := c.Ping(pctx)
			pcancel()
			if err != nil {
				if ctx.Err() == nil {
					end("peer stopped answering pings")
				}
				return
			}
			lastHeard.Store(time.Now().UnixNano())
		}
	}()

	if len(replay) > 0 {
		if err := c.Write(ctx, websocket.MessageBinary, replay); err != nil {
			end("write error")
			return
		}
	}
	for {
		select {
		case b, ok := <-sub.C:
			if !ok {
				if exited, code := sess.Exited(); exited {
					note("session exited") // before closing, so our own close frame is not read as the client's
					msg, _ := json.Marshal(map[string]any{"type": "exit", "code": code})
					_ = c.Write(ctx, websocket.MessageText, msg)
					_ = c.Close(websocket.StatusNormalClosure, "exited")
				} else {
					note("too slow")
					_ = c.Close(websocket.StatusTryAgainLater, "too slow, reattach")
				}
				return
			}
			if err := c.Write(ctx, websocket.MessageBinary, b); err != nil {
				if !errors.Is(err, context.Canceled) {
					end("write error")
				}
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
