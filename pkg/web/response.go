package web

import (
	"encoding/json"
	"io"

	"net/http"
)

// EncodeJSON serializes the response as a JSON object to the ResponseWriter.
// Many JSON-over-HTTP services can use it as a sensible default.
// If the response implements Headerer, the provided headers will be applied to the response.
func EncodeJSON(w http.ResponseWriter, v interface{}, code int) error {
	if headerer, ok := v.(Headerer); ok {
		for k, values := range headerer.Headers() {
			for _, v := range values {
				w.Header().Add(k, v)
			}
		}
	}

	// According to https://tools.ietf.org/search/rfc2616#section-7.2.1:
	//
	// "Any HTTP/1.1 message containing an entity-body SHOULD include a Content-Type
	// header field defining the media type of that body"
	//
	// Since there is no content, then there is no reason to specify a Content-Type header
	if code == http.StatusNoContent {
		w.WriteHeader(code)
		return nil
	}

	var jsonData []byte

	var err error
	switch v := v.(type) {
	case []byte:
		jsonData = v
	case io.Reader:
		jsonData, err = io.ReadAll(v)
	default:
		jsonData, err = json.Marshal(v)
	}

	if err != nil {
		return err
	}

	// Set the content type.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Write the status code to the response and context.
	w.WriteHeader(code)

	// Send the result back to the client.
	if _, err := w.Write(jsonData); err != nil {
		return err
	}

	return nil
}
