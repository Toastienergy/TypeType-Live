package api

import (
	"io"
	"net/http"
	"testing"
)

func (e *env) chapters(t *testing.T, token string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, e.srv.URL+"/v1/videos/123/chapters", nil)
	req.Header.Set(secretHeader, testSecret)
	if token != "" {
		req.Header.Set(tokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func TestVideoChaptersUseTheLinkedAccount(t *testing.T) {
	e := newEnv(t)
	if status := e.chapters(t, "user-token"); status != http.StatusOK {
		t.Fatalf("status %d", status)
	}
	if auth := e.fake.ChapterAuth.Load(); auth != "OAuth user-token" {
		t.Fatalf("authorization %q", auth)
	}
}

func TestVideoChaptersStayAnonymousWithoutAccount(t *testing.T) {
	e := newEnv(t)
	if status := e.chapters(t, ""); status != http.StatusOK {
		t.Fatalf("status %d", status)
	}
	if auth := e.fake.ChapterAuth.Load(); auth != "" {
		t.Fatalf("authorization %q", auth)
	}
}

func TestVideoChaptersFallBackToAnonymousOnRevokedToken(t *testing.T) {
	e := newEnv(t)
	if status := e.chapters(t, "revoked"); status != http.StatusOK {
		t.Fatalf("status %d", status)
	}
	if auth := e.fake.ChapterAuth.Load(); auth != "" {
		t.Fatalf("authorization %q", auth)
	}
}
