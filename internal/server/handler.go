package server

import (
	"fmt"
	"net/http"
)

const boundary = "frame"

// Handler streams PNG frames as multipart/x-mixed-replace.
type Handler struct {
	broker *Broker
}

func NewHandler(b *Broker) *Handler {
	return &Handler{broker: b}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := h.broker.Subscribe()
	defer h.broker.Unsubscribe(ch)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case frame := <-ch:
			fmt.Fprintf(w, "--%s\r\nContent-Type: image/png\r\nContent-Length: %d\r\n\r\n", boundary, len(frame))
			w.Write(frame)
			fmt.Fprintf(w, "\r\n")
			flusher.Flush()
		}
	}
}
