package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dkage/dungeon-master/internal/uploads"
)

// maxUploadBody bounds the JSON body: the largest file, base64-encoded, plus
// its name.
const maxUploadBody = uploads.MaxBytes*4/3 + 4<<10

type uploadReq struct {
	Name string `json:"name"`
	// Data is the file content, base64-encoded.
	Data string `json:"data"`
}

type uploadResp struct {
	// Path is where the file now lives, for the user to hand to the agent.
	Path string `json:"path"`
	Kind string `json:"kind"`
}

// handleUpload stores a file pasted or dropped onto a live session's
// terminal and answers with its path. Nothing leaves the machine.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if s.uploads == nil {
		httpErr(w, http.StatusServiceUnavailable, "uploads are not available")
		return
	}
	id := r.PathValue("id")
	sess, ok := s.cfg.Sessions.Get(id)
	if !ok {
		httpErr(w, http.StatusNotFound, "no such session")
		return
	}
	if exited, _ := sess.Exited(); exited {
		httpErr(w, http.StatusConflict, "the session has ended")
		return
	}
	var req uploadReq
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxUploadBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpErr(w, http.StatusRequestEntityTooLarge, "the file is too large or the request is invalid")
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "invalid file data")
		return
	}
	path, kind, err := s.uploads.Save(id, req.Name, data)
	switch {
	case errors.Is(err, uploads.ErrTooLarge):
		httpErr(w, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, uploads.ErrEmpty), errors.Is(err, uploads.ErrBadSession):
		httpErr(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, uploads.ErrUnsupported):
		httpErr(w, http.StatusUnsupportedMediaType, err.Error())
	case err != nil:
		httpErr(w, http.StatusInternalServerError, "could not store the file")
	default:
		writeJSON(w, http.StatusCreated, uploadResp{Path: path, Kind: kind})
	}
}

func (s *Server) removeUploads(session string) {
	if s.uploads != nil {
		s.uploads.Remove(session)
	}
}
