package rusty

import (
	"io"
	"net/url"
	"strings"

	"github.com/valyala/fasttemplate"
)

const (
	noneEscape int = iota
	queryEscape
	pathEscape
)

// URL returns an url string with the provided elem joined to the existing path of base.
// It may return an empty string if an error occurs.
//
// base is usually the host part of the URL and, optionally, a sequence of path segments.
// elem may contain path segments with a query string, or the query string only (must include ?).
// Examples:
//
//	URL("http://api.server.com/resource/{id}", "/sub-resource?filter={filter}")
//	URL("http://api.server.com", "/resource/{id}?filter={filter}")
//	URL("http://api.server.com", "?filter={filter}")
func URL(base string, elem string) string {
	u, err := url.Parse(base)
	if err != nil {
		return ""
	}

	path, params, found := strings.Cut(elem, "?")
	u2, err := url.Parse(path)
	if err != nil {
		return ""
	}

	if u2.Path != "" {
		u = u.JoinPath(u2.Path)
	}

	if found {
		u.RawQuery = params
	}

	unescapedURL, err := url.PathUnescape(u.String())
	if err != nil {
		return ""
	}

	return unescapedURL
}

func expandURLTemplate(u *url.URL, params map[string]string, query url.Values) (*url.URL, error) {
	u2 := cloneURL(u)
	p, err := fasttemplate.ExecuteFuncStringWithErr(u.Path, "{", "}", func(w io.Writer, tag string) (int, error) { return tagFunc(w, tag, params, noneEscape) })
	if err != nil {
		return nil, err
	}

	rawPath, err := fasttemplate.ExecuteFuncStringWithErr(u.Path, "{", "}", func(w io.Writer, tag string) (int, error) { return tagFunc(w, tag, params, pathEscape) })
	if err != nil {
		return nil, err
	}

	rawQuery, err := fasttemplate.ExecuteFuncStringWithErr(u.RawQuery, "{", "}", func(w io.Writer, tag string) (int, error) { return tagFunc(w, tag, params, queryEscape) })
	if err != nil {
		return nil, err
	}

	if rawQuery != "" && len(query) > 0 {
		rawQuery += "&"
	}

	rawQuery += query.Encode()

	u2.Path = p
	u2.RawPath = rawPath
	u2.RawQuery = rawQuery
	return u2, nil
}

func noopEscape(s string) string { return s }

func tagFunc(w io.Writer, tag string, m map[string]string, mode int) (int, error) {
	escapeFunc := noopEscape
	switch mode {
	case queryEscape:
		escapeFunc = url.QueryEscape
	case pathEscape:
		escapeFunc = url.PathEscape
	}

	v, ok := m[tag]
	if !ok {
		return 0, ErrMissingURLParam
	}

	if v == "" && mode != queryEscape {
		return 0, ErrEmptyURLParam
	}

	return w.Write([]byte(escapeFunc(v)))
}

// cloneURL from stdlib net/http package.
func cloneURL(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	u2 := new(url.URL)
	*u2 = *u
	if u.User != nil {
		u2.User = new(url.Userinfo)
		*u2.User = *u.User
	}
	return u2
}
