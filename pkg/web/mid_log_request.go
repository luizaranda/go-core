package web

import (
	"bytes"
	"github.com/luizaranda/go-core/pkg/log"
	"io"
	"net/http"
)

// wrappedResponseWriter é um wrapper para http.ResponseWriter que captura a resposta
type wrappedResponseWriter struct {
	http.ResponseWriter
	buffer *bytes.Buffer
}

// Write implementa a interface http.ResponseWriter e captura a resposta
func (w *wrappedResponseWriter) Write(b []byte) (int, error) {
	w.buffer.Write(b)
	return w.ResponseWriter.Write(b)
}

// LogRequestConfig allow configuring the way in which the LogRequest middleware
// will behave.
type LogRequestConfig struct {
	IncludeRequest  bool
	IncludeResponse bool
}

// LogRequest allows the logging of the whole request/response.
func LogRequest(logger log.Logger, cfg LogRequestConfig) Middleware {
	// This is the actual middleware function to be executed.
	return func(handler http.HandlerFunc) http.HandlerFunc {
		// Create the innerHandler that will be attached in the middleware chain.
		return func(w http.ResponseWriter, r *http.Request) {
			// Prevent allocations when current log level is not debug level
			if logger.Level() != log.DebugLevel {
				handler(w, r)
				return
			}

			var reqBuf *bytes.Buffer
			if cfg.IncludeRequest {
				// Using the request content length try to preallocate the
				// buffer to store the request body.
				contentLength := int(r.ContentLength)
				reqBuf = bytes.NewBuffer(make([]byte, 0, contentLength))

				// Ensure the request body is closed, if not the connection may hang.
				origBody := r.Body
				defer origBody.Close()

				// By using a TeeReader we ensure we do not block reading the request
				// body only for logging. This works by reading the body when the http
				// handlers reads from it, and filling the buffer with what's read.
				// This has the downside (or not) that if the innerHandler decides to fail
				// before reading the body we won't log it.
				r.Body = io.NopCloser(io.TeeReader(origBody, reqBuf))
			}

			ww := &responseWriter{w: w, status: http.StatusOK}

			var resBuf *bytes.Buffer
			if cfg.IncludeResponse {
				resBuf = bytes.NewBuffer(make([]byte, 0, 1024))

				// Criamos um wrapper para o responseWriter para capturar a resposta
				originalWriter := ww.w
				ww.w = &wrappedResponseWriter{
					ResponseWriter: originalWriter,
					buffer:         resBuf,
				}
			}

			// Execute wrapped handlers with our wrapped ResponseWriter.
			// Capture the error as we want to log it.
			handler(ww, r)

			fields := []log.Field{
				log.String("method", r.Method),
				log.Stringer("url", r.URL),
				log.Int("status", ww.Status()),
			}

			if reqBuf != nil {
				fields = append(fields,
					log.Reflect("request_headers", r.Header),
					log.ByteString("request_body", reqBuf.Bytes()),
				)
			}

			if resBuf != nil {
				fields = append(fields,
					log.Reflect("response_headers", ww.Header()),
					log.ByteString("response_body", resBuf.Bytes()),
				)
			}

			logger.Debug("request handled", fields...)
		}
	}
}
