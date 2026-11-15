package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrUpstream = errors.New("proxy: upstream error")

type StatusError struct {
	Status int
}

func (e *StatusError) Error() string {
	return "proxy: upstream status " + strconv.Itoa(e.Status)
}

const copyBufferSize = 32 * 1024

var passHeaders = []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Last-Modified", "ETag"}

type Relay struct {
	client *http.Client
	hosts  *Hosts
	pool   sync.Pool
}

func NewHTTPClient(hosts *Hosts) *http.Client {
	transport := &http.Transport{
		DialContext:           PublicDialer(5*time.Second, hosts.AllowInsecure).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   64,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return ErrHostNotAllowed
			}
			_, err := hosts.Allowed(req.URL.String())
			return err
		},
	}
}

func NewRelay(client *http.Client, hosts *Hosts) *Relay {
	return &Relay{
		client: client,
		hosts:  hosts,
		pool: sync.Pool{New: func() any {
			buf := make([]byte, copyBufferSize)
			return &buf
		}},
	}
}

func (r *Relay) Fetch(ctx context.Context, upstream string, maxBytes int64) ([]byte, error) {
	resp, err := r.get(ctx, upstream, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &StatusError{Status: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUpstream, err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("%w: body above %d bytes", ErrUpstream, maxBytes)
	}
	return body, nil
}

func (r *Relay) Stream(w http.ResponseWriter, req *http.Request, upstream string) error {
	forward := http.Header{}
	for _, name := range []string{"Range", "If-Range"} {
		if v := req.Header.Get(name); v != "" {
			forward.Set(name, v)
		}
	}
	resp, err := r.get(req.Context(), upstream, forward)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK, http.StatusPartialContent, http.StatusRequestedRangeNotSatisfiable:
	default:
		return &StatusError{Status: resp.StatusCode}
	}
	for _, name := range passHeaders {
		if v := resp.Header.Get(name); v != "" {
			w.Header().Set(name, v)
		}
	}
	if ct := w.Header().Get("Content-Type"); ct == "" || strings.HasSuffix(ct, "octet-stream") {
		w.Header().Set("Content-Type", mediaType(resp.Request.URL.Path))
	}
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.WriteHeader(resp.StatusCode)
	buf := r.pool.Get().(*[]byte)
	defer r.pool.Put(buf)
	_, err = io.CopyBuffer(writerOnly{w}, resp.Body, *buf)
	return err
}

func (r *Relay) get(ctx context.Context, upstream string, headers http.Header) (*http.Response, error) {
	u, err := r.hosts.Allowed(upstream)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header[k] = v
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUpstream, err)
	}
	return resp, nil
}

func mediaType(path string) string {
	switch {
	case strings.HasSuffix(path, ".mp4"):
		return "video/mp4"
	case strings.HasSuffix(path, ".ts"):
		return "video/mp2t"
	}
	return "application/octet-stream"
}

type writerOnly struct {
	io.Writer
}
