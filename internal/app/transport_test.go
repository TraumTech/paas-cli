package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClientSendsBearerTokenWhenSet(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	resp, err := httpClient("secret-token").Get(srv.URL)
	if err != nil {
		t.Fatalf("запрос не удался: %v", err)
	}
	resp.Body.Close()

	if want := "Bearer secret-token"; got != want {
		t.Fatalf("Authorization = %q, ожидался %q", got, want)
	}
}

func TestHTTPClientOmitsAuthorizationWhenTokenEmpty(t *testing.T) {
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, present = r.Header["Authorization"]
	}))
	defer srv.Close()

	client := httpClient("")
	if client.Transport != nil {
		t.Fatalf("без токена кастомный transport не нужен, получили %T", client.Transport)
	}

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("запрос не удался: %v", err)
	}
	resp.Body.Close()

	if present {
		t.Fatal("без токена заголовок Authorization не должен отправляться")
	}
}

func TestBearerTransportDoesNotMutateOriginalRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.test", nil)
	transport := &bearerTransport{token: "tok", base: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})}

	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("исходный запрос мутировал: Authorization = %q", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
