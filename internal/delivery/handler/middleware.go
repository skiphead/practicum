package handlers

import (
	"compress/gzip"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strings"
	"time"
)

func compressionMiddleware(next http.Handler) http.Handler {
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

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		duration := time.Since(start)

		logRequest(r, duration)
		logResponse(ww)
	})
}

func logRequest(r *http.Request, duration time.Duration) {
	zap.L().Info("request",
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
		zap.Duration("duration", duration))
}

func logResponse(ww middleware.WrapResponseWriter) {
	zap.L().Info("response",
		zap.Int("status", ww.Status()),
		zap.Int("bytes", ww.BytesWritten()))
}

func shouldCompressResponse(r *http.Request) bool {
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		return false
	}

	contentType := r.Header.Get("Content-Type")
	return strings.HasPrefix(contentType, "text/plain") ||
		contentType == "application/json"
}

func shouldDecompressRequest(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	supportedContentType := contentType == "text/plain" || contentType == "application/json"
	contentEncoding := r.Header.Get("Content-Encoding")
	sendsGzip := strings.Contains(contentEncoding, "gzip")

	return supportedContentType && sendsGzip
}

type compressWriter struct {
	w  http.ResponseWriter
	zw *gzip.Writer
}

func newCompressWriter(w http.ResponseWriter) *compressWriter {
	return &compressWriter{
		w:  w,
		zw: gzip.NewWriter(w),
	}
}

func (c *compressWriter) Header() http.Header {
	return c.w.Header()
}

func (c *compressWriter) Write(p []byte) (int, error) {
	return c.zw.Write(p)
}

func (c *compressWriter) WriteHeader(statusCode int) {
	c.w.Header().Set("Content-Encoding", "gzip")
	c.w.WriteHeader(statusCode)
}

func (c *compressWriter) Close() error {
	return c.zw.Close()
}

type compressReader struct {
	r  io.ReadCloser
	zr *gzip.Reader
}

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

func (c *compressReader) Read(p []byte) (n int, err error) {
	return c.zr.Read(p)
}

func (c *compressReader) Close() error {
	if err := c.r.Close(); err != nil {
		return err
	}
	return c.zr.Close()
}
