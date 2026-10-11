// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
)

// Error sentinels the raw failures wrap, matchable with errors.Is.
var (
	errUnauthorized = errors.New("unauthorized")
	errNotFound     = errors.New("not found")
)

// httpError carries the response status and a sanitised body snippet.
// It wraps one of the sentinels above when the status maps to one. Every
// failure leaves the raw client inside a *forge.StatusError that wraps it, so a
// caller sees the same shape as for an SDK failure and the message is unchanged.
type httpError struct {
	StatusCode int
	Status     string
	Body       string
	Method     string
	URL        string
	wrapped    error
}

// rawResponse contains a bounded response body and the headers needed to
// resume ranged API reads.
type rawResponse struct {
	Body          []byte
	ContentType   string
	ContentRange  string
	ContentLength int64
	StatusCode    int
}

func (e *httpError) Error() string {
	return fmt.Sprintf("%s %s: %s: %s", e.Method, e.URL, e.Status, e.Body)
}

func (e *httpError) Unwrap() error { return e.wrapped }

// rawClient is the one raw-HTTP path, for the endpoints the SDK lacks. It is
// bound to one instance and one credential, both fixed when the adapter is
// built, so there is no package-level target and adapters never share state.
type rawClient struct {
	base  string // Forgejo URL, without /api/v1
	token string // the request's credential, as the factory resolved it
	ua    string
	hc    *http.Client
}

const defaultUserAgent = "forgejo-mcp/"

// sharedHTTPClient is shared so keep-alives work across tool calls.
var sharedHTTPClient = &http.Client{Timeout: 60 * time.Second}

func newRawClient(base, token, ua string) *rawClient {
	if ua == "" {
		ua = defaultUserAgent
	}
	return &rawClient{base: base, token: token, ua: ua, hc: sharedHTTPClient}
}

// withBase is a copy of the client aimed at another instance.
func (r *rawClient) withBase(base string) *rawClient {
	c := *r
	c.base = base
	return &c
}

// resolveURL turns a path or absolute URL into an absolute URL string.
// API paths (e.g. "/repos/x/y/issues/1/assets") are prefixed with
// the Forgejo URL + "/api/v1". Absolute URLs are returned verbatim.
func (r *rawClient) resolveURL(pathOrURL string) (string, error) {
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		return pathOrURL, nil
	}
	base := strings.TrimRight(r.base, "/")
	if base == "" {
		return "", errors.New("the Forgejo URL is empty; the raw-HTTP client needs a configured base URL")
	}
	if !strings.HasPrefix(pathOrURL, "/") {
		pathOrURL = "/" + pathOrURL
	}
	// Forgejo REST API root.
	return base + "/api/v1" + pathOrURL, nil
}

// resolveSameOriginURL is like resolveURL but for asset/download URLs that
// live outside the /api/v1 prefix (e.g. /attachments/{uuid}). Absolute URLs
// pass through; relative URLs hang off the Forgejo URL with no /api/v1.
func (r *rawClient) resolveSameOriginURL(pathOrURL string) (string, error) {
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		return pathOrURL, nil
	}
	base := strings.TrimRight(r.base, "/")
	if base == "" {
		return "", errors.New("the Forgejo URL is empty; the raw-HTTP client needs a configured base URL")
	}
	if !strings.HasPrefix(pathOrURL, "/") {
		pathOrURL = "/" + pathOrURL
	}
	return base + pathOrURL, nil
}

func (r *rawClient) setCommonHeaders(req *http.Request) {
	req.Header.Set("Authorization", "token "+r.token)
	req.Header.Set("User-Agent", r.ua)
	req.Header.Set("Accept", "application/json")
}

// doRequest sends req and returns the response. Caller owns response body close.
func (r *rawClient) doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := r.hc.Do(req)
	duration := time.Since(start)
	endpoint := req.URL.Path
	if req.URL.RawQuery != "" {
		endpoint += "?" + req.URL.RawQuery
	}
	if err != nil {
		forge.LogAPICall(ctx, req.Method, endpoint, duration, 0, err)
		return nil, fmt.Errorf("%s %s: %w", req.Method, req.URL.String(), err)
	}
	forge.LogAPICall(ctx, req.Method, endpoint, duration, resp.StatusCode, nil)
	return resp, nil
}

// readBodySnippet reads up to 1 KiB of the body for inclusion in errors.
// It does not close the body.
func readBodySnippet(r io.Reader) string {
	buf := make([]byte, 1024)
	n, _ := io.ReadFull(io.LimitReader(r, 1024), buf)
	return string(buf[:n])
}

// httpErrorFromResponse builds the failure for a non-2xx response: a
// *forge.StatusError carrying the status, wrapping an *httpError that maps
// 401/403/404 to the sentinels.
func httpErrorFromResponse(req *http.Request, resp *http.Response) error {
	e := &httpError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Body:       readBodySnippet(resp.Body),
		Method:     req.Method,
		URL:        req.URL.String(),
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		e.wrapped = errUnauthorized
	case http.StatusNotFound:
		e.wrapped = errNotFound
	}
	return &forge.StatusError{StatusCode: resp.StatusCode, Err: e}
}

// doJSON performs an authenticated JSON request. Encodes body as JSON if
// non-nil; decodes 2xx response into out if non-nil. 204 responses are
// always success-with-no-body. 4xx/5xx return a *forge.StatusError.
//
// For list endpoints where a 404 should mean "empty list" (Forgejo's habit
// when the parent entity has no children), use doJSONList instead.
func (r *rawClient) doJSON(ctx context.Context, method, pathOrURL string, body, out any) error {
	_, err := r.doJSONWithHeader(ctx, method, pathOrURL, body, out)
	return err
}

// doJSONWithHeader is doJSON plus the response headers. 4xx/5xx remain
// errors, including 404 -- unlike doJSONList, a missing parent is not
// rewritten as an empty list. Paged list endpoints need the headers: Forgejo
// reports the total in X-Total-Count and "another page exists" in
// Link ... rel="next", and a bounded resource that must not over-fetch has no
// other way to detect truncation. The header is returned even on error when a
// response was received.
func (r *rawClient) doJSONWithHeader(ctx context.Context, method, pathOrURL string, body, out any) (http.Header, error) {
	full, err := r.resolveURL(pathOrURL)
	if err != nil {
		return nil, err
	}
	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, full, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	r.setCommonHeaders(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := r.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.Header, httpErrorFromResponse(req, resp)
	}
	if resp.StatusCode == http.StatusNoContent || out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.Header, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return resp.Header, fmt.Errorf("decode response: %w", err)
	}
	return resp.Header, nil
}

// doJSONList is like doJSON but treats 404 as "empty list" (no error,
// out left at its zero value).
func (r *rawClient) doJSONList(ctx context.Context, method, pathOrURL string, out any) error {
	_, err := r.doJSONListWithHeader(ctx, method, pathOrURL, out)
	return err
}

// doJSONListWithHeader is doJSONList plus the response headers. On the
// 404-means-empty path the headers are still returned.
func (r *rawClient) doJSONListWithHeader(ctx context.Context, method, pathOrURL string, out any) (http.Header, error) {
	header, err := r.doJSONWithHeader(ctx, method, pathOrURL, nil, out)
	if forge.IsNotFound(err) {
		return header, nil
	}
	return header, err
}

// doAPIRaw performs an authenticated GET against an API path and reads at most
// maxBytes. The optional byteRange value is sent as the HTTP Range header.
// Callers must use response range metadata to make bounded reads resumable.
func (r *rawClient) doAPIRaw(ctx context.Context, pathOrURL, accept, byteRange string, maxBytes int64) (*rawResponse, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("maxBytes must be positive")
	}
	full, err := r.resolveURL(pathOrURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	r.setCommonHeaders(req)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if byteRange != "" {
		req.Header.Set("Range", byteRange)
	}

	resp, err := r.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpErrorFromResponse(req, resp)
	}

	limited := io.LimitReader(resp.Body, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, forge.ErrPayloadTooLarge
	}

	return &rawResponse{
		Body:          body,
		ContentType:   resp.Header.Get("Content-Type"),
		ContentRange:  resp.Header.Get("Content-Range"),
		ContentLength: resp.ContentLength,
		StatusCode:    resp.StatusCode,
	}, nil
}

// quoteEscaper mirrors mime/multipart's internal escaper for filenames.
var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

// doMultipart uploads a single file part via multipart/form-data and
// decodes the JSON response into out (if non-nil).
func (r *rawClient) doMultipart(ctx context.Context, method, pathOrURL, fieldName, filename, mimeType string, body io.Reader, out any) error {
	full, err := r.resolveURL(pathOrURL)
	if err != nil {
		return err
	}

	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	contentType := multipartWriter.FormDataContentType()

	req, err := http.NewRequestWithContext(ctx, method, full, pipeReader)
	if err != nil {
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
		return fmt.Errorf("build request: %w", err)
	}
	go writeMultipart(pipeWriter, multipartWriter, fieldName, filename, mimeType, body)

	r.setCommonHeaders(req)
	req.Header.Set("Content-Type", contentType)
	resp, err := r.doRequest(ctx, req)
	_ = pipeReader.Close()
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpErrorFromResponse(req, resp)
	}
	if resp.StatusCode == http.StatusNoContent || out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func writeMultipart(pipeWriter *io.PipeWriter, multipartWriter *multipart.Writer, fieldName, filename, mimeType string, r io.Reader) {
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
		quoteEscaper.Replace(fieldName), quoteEscaper.Replace(filename)))
	if mimeType != "" {
		h.Set("Content-Type", mimeType)
	} else {
		h.Set("Content-Type", "application/octet-stream")
	}
	part, err := multipartWriter.CreatePart(h)
	if err == nil {
		_, err = io.Copy(part, r)
	}
	if err == nil {
		err = multipartWriter.Close()
	}
	if err != nil {
		_ = pipeWriter.CloseWithError(err)
		return
	}
	_ = pipeWriter.Close()
}

// doRaw fetches bytes from a URL (absolute or relative-to-the-Forgejo-URL with no
// /api/v1 prefix), adding the credential. Caps the response at
// forge.MaxInlineDownloadBytes; forge.ErrPayloadTooLarge is returned if the
// body would exceed the cap. Returns body bytes + content type.
func (r *rawClient) doRaw(ctx context.Context, pathOrURL string) ([]byte, string, error) {
	full, err := r.resolveSameOriginURL(pathOrURL)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build request: %w", err)
	}
	r.setCommonHeaders(req)

	resp, err := r.doRequest(ctx, req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", httpErrorFromResponse(req, resp)
	}

	// Read up to cap+1 to detect overflow.
	limited := io.LimitReader(resp.Body, forge.MaxInlineDownloadBytes+1)
	buf, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", fmt.Errorf("read body: %w", err)
	}
	if int64(len(buf)) > forge.MaxInlineDownloadBytes {
		return nil, "", forge.ErrPayloadTooLarge
	}
	return buf, resp.Header.Get("Content-Type"), nil
}
