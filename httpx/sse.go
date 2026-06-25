package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SSEWriter wraps a gin.ResponseWriter for Server-Sent Events streaming.
// It handles SSE header setup, event formatting, flushing, and the [DONE] protocol.
type SSEWriter struct {
	rw gin.ResponseWriter
}

// NewSSEWriter creates an SSEWriter and immediately sets the standard SSE headers
// (Content-Type, Cache-Control, Connection, X-Accel-Buffering).
func NewSSEWriter(c *gin.Context) *SSEWriter {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	return &SSEWriter{rw: c.Writer}
}

// WriteEvent writes a named SSE event with JSON-encoded data and flushes.
// Format: "event: {name}\ndata: {json}\n\n"
func (w *SSEWriter) WriteEvent(event string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("sse: marshal event data: %w", err)
	}
	if _, err := fmt.Fprintf(w.rw, "event: %s\ndata: %s\n\n", event, b); err != nil {
		return fmt.Errorf("sse: write event: %w", err)
	}
	w.Flush()
	return nil
}

// WriteData writes raw data without an event name and flushes.
// Format: "data: {raw}\n\n"
func (w *SSEWriter) WriteData(raw string) error {
	if _, err := fmt.Fprintf(w.rw, "data: %s\n\n", raw); err != nil {
		return fmt.Errorf("sse: write data: %w", err)
	}
	w.Flush()
	return nil
}

// WriteDone writes the standard stream termination marker and flushes.
// Format: "data: [DONE]\n\n"
func (w *SSEWriter) WriteDone() {
	_, _ = fmt.Fprintf(w.rw, "data: [DONE]\n\n")
	w.Flush()
}

// WriteError writes an error event with a JSON envelope and flushes.
// Format: "event: error\ndata: {\"code\":500,\"msg\":\"...\"}\n\n"
func (w *SSEWriter) WriteError(err error) {
	payload := Response{Code: 500, Msg: err.Error()}
	b, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(w.rw, "event: error\ndata: %s\n\n", b)
	w.Flush()
}

// Flush flushes the underlying ResponseWriter if it supports http.Flusher.
func (w *SSEWriter) Flush() {
	if f, ok := w.rw.(http.Flusher); ok {
		f.Flush()
	}
}

// Writer returns the underlying http.ResponseWriter.
// Useful for passing to components like AgentSSEBridge.StreamTo that need a raw writer.
func (w *SSEWriter) Writer() http.ResponseWriter {
	return w.rw
}
