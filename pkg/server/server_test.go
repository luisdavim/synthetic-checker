package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServer(t *testing.T) {
	var cfg Config
	srv := New(cfg)
	srv.WithRoutes(Routes{})
	req, err := http.NewRequest("GET", "/healthz", bytes.NewBuffer([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	handler := http.HandlerFunc(srv.livelinessHandler)
	handler.ServeHTTP(w, req)

	if status := w.Code; status != http.StatusOK {
		t.Errorf("got %v, expected %v", status, http.StatusOK)
	}
}
