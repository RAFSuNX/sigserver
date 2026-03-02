package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"live-sys-stats/internal/server"
)

func TestHandlerContentType(t *testing.T) {
	b := server.NewBroker()
	h := server.NewHandler(b)

	req := httptest.NewRequest(http.MethodGet, "/sig.png", nil)
	rec := httptest.NewRecorder()

	// Run handler in goroutine since it streams indefinitely
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(rec, req)
		close(done)
	}()

	// Give it a moment to write headers
	time.Sleep(50 * time.Millisecond)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "multipart/x-mixed-replace") {
		t.Errorf("expected multipart content-type, got: %s", ct)
	}
}

func TestBrokerPublishReachesSubscriber(t *testing.T) {
	b := server.NewBroker()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	frame := []byte("test-frame")
	b.Publish(frame)

	select {
	case got := <-ch:
		if string(got) != string(frame) {
			t.Errorf("expected %q, got %q", frame, got)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for published frame")
	}
}
