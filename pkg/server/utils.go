package server

import (
	"bytes"
	"encoding/json"
	"net/http"
)

func (s *Server) JSONResponse(w http.ResponseWriter, r *http.Request, result interface{}, responseCode int) {
	body, err := json.Marshal(result)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.logger.Err(err).Msg("JSON marshal failed")
		return
	}
	if s.config.PrettyJSON {
		body = PrettyJSON(body)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(responseCode)
	if _, err := w.Write(body); err != nil {
		s.logger.Err(err).Msg("failed to write response")
	}
}

func PrettyJSON(b []byte) []byte {
	var out bytes.Buffer
	if err := json.Indent(&out, b, "", "  "); err != nil {
		return b
	}
	return out.Bytes()
}
