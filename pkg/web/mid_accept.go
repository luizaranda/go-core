package web

import (
	"mime"
	"net/http"
	"regexp"
	"strings"
)

const _all = "*/*"

// AcceptJSON makes only application/json, */* or empty Accept header values
// acceptable requests. The rest will get a NotAcceptable response.
func AcceptJSON() Middleware {
	return Accept("^application/json$")
}

// Accept makes acceptable any request whose Accept header
// matches any of the mediaTypes regular expressions.
// This function will panic if any of the mediaTypes expressions is not valid.
//
// Example:
// app.Router.Get("/",handler,web.Accept("^image/.+","^application/pdf*")).
func Accept(mediaTypes ...string) Middleware {
	compiled := make([]*regexp.Regexp, len(mediaTypes))
	for i := 0; i < len(mediaTypes); i++ {
		compiled[i] = regexp.MustCompile(mediaTypes[i])
	}

	return func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !acceptable(r.Header.Get("Accept"), compiled) {
				w.WriteHeader(http.StatusNotAcceptable)
				return
			}
			handler(w, r)
		}
	}
}

func acceptable(accept string, mediaTypes []*regexp.Regexp) bool {
	if accept == "" || accept == _all {
		// The absence of an Accept header is equivalent to "*/*".
		// https://tools.ietf.org/html/rfc2296#section-4.2.2
		return true
	}

	for _, a := range strings.Split(accept, ",") {
		mediaType, _, err := mime.ParseMediaType(a)
		if err != nil {
			continue
		}

		if mediaType == _all {
			return true
		}

		for _, t := range mediaTypes {
			if t.MatchString(mediaType) {
				return true
			}
		}
	}

	return false
}
