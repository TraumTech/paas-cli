package citokengithub

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TraumTech/paas-cli/internal/entities"
)

func fakeJWT(payload string) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"RS256"}`)) + "." + enc([]byte(payload)) + ".sig"
}

func TestNewOutsideGitHubActions(t *testing.T) {
	env := map[string]string{EnvRequestURL: "https://example"}
	if _, ok := New(func(k string) string { return env[k] }, "aud", http.DefaultClient); ok {
		t.Fatal("без ключа запроса источник не должен собираться")
	}
}

func TestTokenRequestsAudienceAndReadsExpiry(t *testing.T) {
	token := fakeJWT(`{"exp":1900000000,"repository_id":"1"}`)
	var gotAudience, gotAuth, gotExisting string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAudience = r.URL.Query().Get("audience")
		gotExisting = r.URL.Query().Get("api-version")
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"value":"` + token + `"}`))
	}))
	defer srv.Close()
	env := map[string]string{EnvRequestURL: srv.URL + "/token?api-version=2.0", EnvRequestToken: "req-secret"}
	source, ok := New(func(k string) string { return env[k] }, "https://api.paas.traumtech.ru", srv.Client())
	if !ok {
		t.Fatal("источник должен собраться")
	}

	credential, err := source.Token(context.Background())

	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if gotAudience != "https://api.paas.traumtech.ru" || gotExisting != "2.0" {
		t.Fatalf("query: audience=%q api-version=%q", gotAudience, gotExisting)
	}
	if gotAuth != "Bearer req-secret" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if credential.Token != token || credential.Kind != entities.CredentialCI {
		t.Fatalf("credential = %+v", credential)
	}
	if !credential.ExpiresAt.Equal(time.Unix(1900000000, 0)) {
		t.Fatalf("ExpiresAt = %v", credential.ExpiresAt)
	}
}

func TestTokenFailsOnDeniedRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	env := map[string]string{EnvRequestURL: srv.URL, EnvRequestToken: "x"}
	source, _ := New(func(k string) string { return env[k] }, "aud", srv.Client())

	if _, err := source.Token(context.Background()); err == nil {
		t.Fatal("отказ GitHub должен быть ошибкой")
	}
}

func TestExpiresAtUnparsableIsZero(t *testing.T) {
	if !expiresAt("not-a-jwt").IsZero() {
		t.Fatal("неразборный токен — нулевой срок")
	}
}
