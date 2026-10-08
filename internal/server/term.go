package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"
)

// Terminal websocket protocol.
//
//	server → client: binary frames are terminal output (the first one is the
//	retained scrollback); one final text frame {"type":"exit","code":N} when
//	the process ends.
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

	replay, sub := sess.Attach()
	defer sess.Detach(sub)

	// Reader: keystrokes and control messages.
	go func() {
		defer cancel()
		for {
			typ, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			switch typ {
			case websocket.MessageBinary:
				if _, err := sess.Write(data); err != nil {
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

	if len(replay) > 0 {
		if err := c.Write(ctx, websocket.MessageBinary, replay); err != nil {
			return
		}
	}
	for {
		select {
		case b, ok := <-sub.C:
			if !ok {
				if exited, code := sess.Exited(); exited {
					msg, _ := json.Marshal(map[string]any{"type": "exit", "code": code})
					_ = c.Write(ctx, websocket.MessageText, msg)
					_ = c.Close(websocket.StatusNormalClosure, "exited")
				} else {
					_ = c.Close(websocket.StatusTryAgainLater, "too slow, reattach")
				}
				return
			}
			if err := c.Write(ctx, websocket.MessageBinary, b); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
