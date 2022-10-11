package rpc

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

var DefaultOptions = []Option{
	WithContentEncoding(),
}

type Option interface {
	apply(*Procedure) error
}

func WithContentEncoding() Option {
	return ContentEncoding
}

var ContentEncoding Option = contentEncoding{}

type contentEncoding struct{}

func (contentEncoding) apply(p *Procedure) error {
	p.middleware = append(p.middleware, contentEncodingMiddleware)
	return nil
}

func contentEncodingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {

		acceptEncoding := req.Header.Get("Accept-Encoding")

		if strings.Contains(acceptEncoding, "gzip") {
			resp.Header().Set("Content-Encoding", "gzip")
			w := gzip.NewWriter(resp)
			wrapper := &wrappedResponseWriter{w, resp}
			next.ServeHTTP(wrapper, req)
			w.Close()
			return
		}

		if strings.Contains(acceptEncoding, "deflate") {
			resp.Header().Set("Content-Encoding", "deflate")
			w, err := flate.NewWriter(resp, flate.DefaultCompression)
			if err != nil {
				// flate.NewWriter does not throw an error for DefaultCompression
				panic(err)
			}
			wrapper := &wrappedResponseWriter{w, resp}
			next.ServeHTTP(wrapper, req)
			w.Close()
			return
		}

		// no compression
		next.ServeHTTP(resp, req)
	})
}

type wrappedResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *wrappedResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}
