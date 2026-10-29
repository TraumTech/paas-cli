package oauthclienthydra

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/entities"
)

func idToken(claims map[string]any) string {
	payload, _ := json.Marshal(claims)
	return "eyJhbGciOiJSUzI1NiJ9." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func tokenServer(t *testing.T, handle func(form url.Values) (int, any)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, r.ParseForm())
		status, body := handle(r.PostForm)
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
}

func TestAuthorizeURL(t *testing.T) {
	got, err := url.Parse(New("https://oauth.example/", "paas-cli", nil).AuthorizeURL("http://127.0.0.1:5000/callback", "st", "ch"))

	require.NoError(t, err)
	assert.Equal(t, "https://oauth.example/oauth2/auth", got.Scheme+"://"+got.Host+got.Path)
	q := got.Query()
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "paas-cli", q.Get("client_id"))
	assert.Equal(t, "http://127.0.0.1:5000/callback", q.Get("redirect_uri"))
	assert.Equal(t, "openid offline_access", q.Get("scope"))
	assert.Equal(t, "st", q.Get("state"))
	assert.Equal(t, "ch", q.Get("code_challenge"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
}

func TestExchange_ReturnsCredentialWithEmail(t *testing.T) {
	var got url.Values
	srv := tokenServer(t, func(form url.Values) (int, any) {
		got = form
		return http.StatusOK, map[string]any{
			"access_token": "access-1", "refresh_token": "refresh-1", "expires_in": 600,
			"id_token": idToken(map[string]any{"sub": "u-1", "email": "user@example.com"}),
		}
	})
	defer srv.Close()
	client := New(srv.URL, "paas-cli", srv.Client())
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	client.now = func() time.Time { return now }

	credential, err := client.Exchange(context.Background(), "code-1", "verifier-1", "http://127.0.0.1:5000/callback")

	require.NoError(t, err)
	assert.Equal(t, "authorization_code", got.Get("grant_type"))
	assert.Equal(t, "code-1", got.Get("code"))
	assert.Equal(t, "verifier-1", got.Get("code_verifier"))
	assert.Equal(t, "http://127.0.0.1:5000/callback", got.Get("redirect_uri"))
	assert.Equal(t, "paas-cli", got.Get("client_id"))
	assert.Equal(t, entities.CredentialOIDC, credential.Kind)
	assert.Equal(t, "access-1", credential.Token)
	assert.Equal(t, "refresh-1", credential.RefreshToken)
	assert.Equal(t, "user@example.com", credential.Email)
	assert.True(t, now.Add(10*time.Minute).Equal(credential.ExpiresAt))
}

func TestRefresh_RotatesPair(t *testing.T) {
	var got url.Values
	srv := tokenServer(t, func(form url.Values) (int, any) {
		got = form
		return http.StatusOK, map[string]any{"access_token": "access-2", "refresh_token": "refresh-2", "expires_in": 600}
	})
	defer srv.Close()

	credential, err := New(srv.URL, "paas-cli", srv.Client()).Refresh(context.Background(), "refresh-1")

	require.NoError(t, err)
	assert.Equal(t, "refresh_token", got.Get("grant_type"))
	assert.Equal(t, "refresh-1", got.Get("refresh_token"))
	assert.Equal(t, "access-2", credential.Token)
	assert.Equal(t, "refresh-2", credential.RefreshToken)
	assert.Empty(t, credential.Email, "без id_token e-mail неизвестен")
}

// invalid_grant — продлевать нечем: refresh истёк, отозван или уже использован.
func TestRefresh_InvalidGrant_IsExpiredLogin(t *testing.T) {
	srv := tokenServer(t, func(url.Values) (int, any) {
		return http.StatusBadRequest, map[string]any{"error": "invalid_grant", "error_description": "token inactive"}
	})
	defer srv.Close()

	_, err := New(srv.URL, "paas-cli", srv.Client()).Refresh(context.Background(), "refresh-old")

	assert.ErrorIs(t, err, entities.ErrSessionExpired)
}

func TestRefresh_OtherError(t *testing.T) {
	srv := tokenServer(t, func(url.Values) (int, any) {
		return http.StatusUnauthorized, map[string]any{"error": "invalid_client", "error_description": "unknown"}
	})
	defer srv.Close()

	_, err := New(srv.URL, "paas-cli", srv.Client()).Refresh(context.Background(), "refresh-1")

	require.Error(t, err)
	assert.NotErrorIs(t, err, entities.ErrSessionExpired)
	assert.Contains(t, err.Error(), "invalid_client")
}

func TestRevoke(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/oauth2/revoke", r.URL.Path)
		require.NoError(t, r.ParseForm())
		got = r.PostForm
	}))
	defer srv.Close()

	err := New(srv.URL, "paas-cli", srv.Client()).Revoke(context.Background(), "refresh-1")

	require.NoError(t, err)
	assert.Equal(t, "refresh-1", got.Get("token"))
	assert.Equal(t, "refresh_token", got.Get("token_type_hint"))
	assert.Equal(t, "paas-cli", got.Get("client_id"))
}

func TestEmailClaim_Malformed(t *testing.T) {
	assert.Empty(t, emailClaim(""))
	assert.Empty(t, emailClaim("a.b"))
	assert.Empty(t, emailClaim("a.!!!.c"))
	assert.Empty(t, emailClaim(idToken(map[string]any{"sub": "u-1"})))
}
