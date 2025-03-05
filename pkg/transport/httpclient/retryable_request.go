package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

// GetBodyFunc is the function signature of http.Request.GetBody method.
type GetBodyFunc func() (io.ReadCloser, error)

// ReaderFunc is a type of function that can be given natively to NewRequest. It
// can be easily converted into a GetBodyFunc.
type ReaderFunc func() (io.Reader, error)

// GetBodyFunc decorates a ReaderFunc to be compatible with GetBodyFunc.
func (r ReaderFunc) GetBodyFunc() (io.ReadCloser, error) {
	tmp, err := r()
	if err != nil {
		return nil, err
	}
	return io.NopCloser(tmp), nil
}

// lenReader is an interface implemented by many in-memory io.Reader's. Used
// for automatically sending the right Content-Length header when possible.
type lenReader interface{ Len() int }

// NewRequest creates a new retryable http.Request.
//
// rawBody allows many types on readers, it then tries to create the optimal
// rewindable reader depending on the type given.
//
// Optimal body types are either GetBodyFunc or ReaderFunc.
// If rawBody is nil, we use http.NewRequestWithContext directly.
func NewRequest(ctx context.Context, method, url string, rawBody any) (*http.Request, error) {
	if rawBody == nil {
		return http.NewRequestWithContext(ctx, method, url, nil)
	}

	readerFunc, contentLength, err := getBodyReaderAndContentLength(rawBody)
	if err != nil {
		return nil, err
	}

	// We might be able to avoid this call by using a wrapped io.Reader
	// that calls readerFunc lazily after the first call to read.
	bodyReader, err := readerFunc()
	if err != nil {
		return nil, err
	}

	// Using the bodyReader create a new request object and set
	// its ContentLength to the known body value.
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.ContentLength = contentLength

	// GetBody function by definition must return an io.ReadCloser that allows re-reading
	// the request body from the beginning. This allows the standard library to retry
	// requests under some circumstances. We are going to use this function
	// to be able to make full body retries on request execution.
	req.GetBody = readerFunc.GetBodyFunc

	return req, nil
}

func getBodyReaderAndContentLength(rawBody interface{}) (ReaderFunc, int64, error) {
	var bodyReader ReaderFunc
	var contentLength int64

	switch body := rawBody.(type) {
	// If they gave us a function already, great! Use it.
	case ReaderFunc:
		bodyReader = body
		tmp, err := body()
		if err != nil {
			return nil, 0, err
		}
		if lr, ok := tmp.(lenReader); ok {
			contentLength = int64(lr.Len())
		}
		if c, ok := tmp.(io.Closer); ok {
			_ = c.Close()
		}

	case func() (io.Reader, error):
		bodyReader = body
		tmp, err := body()
		if err != nil {
			return nil, 0, err
		}
		if lr, ok := tmp.(lenReader); ok {
			contentLength = int64(lr.Len())
		}
		if c, ok := tmp.(io.Closer); ok {
			_ = c.Close()
		}

	// If a regular byte slice, we can read it over and over via new
	// readers
	case []byte:
		buf := body
		bodyReader = func() (io.Reader, error) {
			return bytes.NewReader(buf), nil
		}
		contentLength = int64(len(buf))

	// If a bytes.Buffer we can read the underlying byte slice over and
	// over
	case *bytes.Buffer:
		buf := body
		bodyReader = func() (io.Reader, error) {
			return bytes.NewReader(buf.Bytes()), nil
		}
		contentLength = int64(buf.Len())

	// We prioritize *bytes.Reader here because we don't really want to
	// deal with it seeking so want it to match here instead of the
	// io.ReadSeeker case.
	case *bytes.Reader:
		snapshot := *body
		bodyReader = func() (io.Reader, error) {
			r := snapshot
			return &r, nil
		}
		contentLength = int64(body.Len())

	// Compat case
	case io.ReadSeeker:
		raw := body
		bodyReader = func() (io.Reader, error) {
			_, err := raw.Seek(0, 0)
			return io.NopCloser(raw), err
		}
		if lr, ok := raw.(lenReader); ok {
			contentLength = int64(lr.Len())
		}

	// Read all in so we can reset
	case io.Reader:
		buf, err := io.ReadAll(body)
		if err != nil {
			return nil, 0, err
		}

		if len(buf) == 0 {
			bodyReader = func() (io.Reader, error) {
				return http.NoBody, nil
			}
			contentLength = 0
		} else {
			bodyReader = func() (io.Reader, error) {
				return bytes.NewReader(buf), nil
			}
			contentLength = int64(len(buf))
		}

	default:
		return nil, 0, fmt.Errorf("cannot handle type %T", rawBody)
	}

	return bodyReader, contentLength, nil
}
