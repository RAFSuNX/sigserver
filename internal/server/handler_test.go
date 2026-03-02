package server_test

import (
	"context"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/sig.png", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.ServeHTTP(rec, req)
	}()

	// Give handler a moment to write headers
	time.Sleep(50 * time.Millisecond)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "multipart/x-mixed-replace") {
		t.Errorf("expected multipart content-type, got: %s", ct)
	}

	// Cancel and wait for goroutine to exit cleanly
	cancel()
	<-done
}

func TestHandlerWritesMultipartFrame(t *testing.T) {
	b := server.NewBroker()
	h := server.NewHandler(b)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/sig.png", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.ServeHTTP(rec, req)
	}()

	// Wait for the handler goroutine to subscribe before publishing
	time.Sleep(20 * time.Millisecond)

	// Publish a known frame
	frame := []byte("fake-png-bytes")
	b.Publish(frame)

	// Wait for frame to be written
	time.Sleep(50 * time.Millisecond)

	// Stop the handler
	cancel()
	<-done

	body := rec.Body.String()
	if !strings.Contains(body, "--frame\r\n") {
		t.Errorf("expected boundary --frame in body, got: %q", body)
	}
	if !strings.Contains(body, "Content-Type: image/png") {
		t.Errorf("expected Content-Type: image/png in body, got: %q", body)
	}
	if !strings.Contains(body, string(frame)) {
		t.Errorf("expected frame bytes in body, got: %q", body)
	}
}

func TestHandlerUnsubscribesOnDisconnect(t *testing.T) {
	b := server.NewBroker()
	h := server.NewHandler(b)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/sig.png", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.ServeHTTP(rec, req)
	}()

	time.Sleep(20 * time.Millisecond)

	// Cancel and wait — Unsubscribe defer should have run
	cancel()
	<-done

	// Publishing after disconnect should not block or panic
	b.Publish([]byte("after-disconnect"))
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
