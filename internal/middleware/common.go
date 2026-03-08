// Package handler provides HTTP middleware and handler for the URL shortening service.
// It includes compression, logging, authentication, and other HTTP-level functionality
// to enhance request/response processing and observability.
package handler

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"time"

	_ "net/http/pprof" // Import pprof for profiling endpoints

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// CompressionMiddleware provides transparent gzip compression/decompression for HTTP requests.
// It compresses responses when clients accept gzip encoding and decompresses requests
// when they are sent with gzip content encoding.
//
// The middleware:
//  1. Checks if client accepts gzip encoding and compresses responses accordingly
//  2. Detects gzip-encoded request bodies and decompresses them transparently
//  3. Sets appropriate Content-Encoding headers for compressed responses
//  4. Properly cleans up compression resources using defer statements
func CompressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ow := w

		if shouldCompressResponse(r) {
			cw := newCompressWriter(w)
			ow = cw
			defer func(cw *compressWriter) {
				errCw := cw.Close()
				if errCw != nil {
					zap.L().Error("error closing compress writer", zap.Error(errCw))
				}
			}(cw)
		}

		if shouldDecompressRequest(r) {
			cr, err := newCompressReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			r.Body = cr
			defer func(cr *compressReader) {
				errCr := cr.Close()
				if errCr != nil {
					zap.L().Error("error closing compress reader", zap.Error(errCr))
				}
			}(cr)
		}

		next.ServeHTTP(ow, r)
	})
}

// LoggingMiddleware provides structured logging for HTTP requests and responses.
// It logs request details (path, method, duration) and response details (status, bytes written).
// This middleware uses Zap for structured logging and Chi's response wrapper for tracking.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		duration := time.Since(start)

		logRequest(r, duration)
		logResponse(ww)
	})
}

// logRequest logs details about an incoming HTTP request.
// It captures the request path, HTTP method, and processing duration.
func logRequest(r *http.Request, duration time.Duration) {
	zap.L().Info("request",
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
		zap.Duration("duration", duration))
}

// logResponse logs details about an outgoing HTTP response.
// It captures the HTTP status code and number of bytes written.
func logResponse(ww middleware.WrapResponseWriter) {
	zap.L().Info("response",
		zap.Int("status", ww.Status()),
		zap.Int("bytes", ww.BytesWritten()))
}

// shouldCompressResponse determines if the response should be compressed.
// Returns true if the client's Accept-Encoding header includes "gzip".
func shouldCompressResponse(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
}

// shouldDecompressRequest determines if the request body should be decompressed.
// Returns true if:
//   - Content-Type is text/plain or application/json
//   - Content-Encoding header includes "gzip"
func shouldDecompressRequest(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	supportedContentType := contentType == "text/plain" || contentType == "application/json"
	contentEncoding := r.Header.Get("Content-Encoding")
	sendsGzip := strings.Contains(contentEncoding, "gzip")

	return supportedContentType && sendsGzip
}

// compressWriter wraps an http.ResponseWriter to provide gzip compression.
// It transparently compresses data written to the response.
type compressWriter struct {
	w  http.ResponseWriter
	zw *gzip.Writer
}

// newCompressWriter creates a new compressWriter instance.
func newCompressWriter(w http.ResponseWriter) *compressWriter {
	return &compressWriter{
		w:  w,
		zw: gzip.NewWriter(w),
	}
}

// Header returns the header map that will be sent by WriteHeader.
func (c *compressWriter) Header() http.Header {
	return c.w.Header()
}

// Write writes compressed data to the underlying gzip writer.
func (c *compressWriter) Write(p []byte) (int, error) {
	return c.zw.Write(p)
}

// WriteHeader sends an HTTP response header with the gzip Content-Encoding.
func (c *compressWriter) WriteHeader(statusCode int) {
	c.w.Header().Set("Content-Encoding", "gzip")
	c.w.WriteHeader(statusCode)
}

// Close closes the gzip writer, flushing any pending compressed data.
func (c *compressWriter) Close() error {
	return c.zw.Close()
}

// compressReader wraps an io.ReadCloser to provide gzip decompression.
// It transparently decompresses data read from the request body.
type compressReader struct {
	r  io.ReadCloser
	zr *gzip.Reader
}

// newCompressReader creates a new compressReader instance.
// Returns an error if the gzip reader cannot be created.
func newCompressReader(r io.ReadCloser) (*compressReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &compressReader{
		r:  r,
		zr: zr,
	}, nil
}

// Read reads decompressed data from the underlying gzip reader.
func (c *compressReader) Read(p []byte) (n int, err error) {
	return c.zr.Read(p)
}

// Close closes both the original reader and the gzip reader.
func (c *compressReader) Close() error {
	if err := c.r.Close(); err != nil {
		return err
	}
	return c.zr.Close()
}
