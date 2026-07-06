package sessiongatewayhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/entities"
)

func TestAuthenticate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/self-service/login/api":
			json.NewEncoder(w).Encode(map[string]any{"id": "flow-1"})
		case r.Method == http.MethodPost && r.URL.Path == "/self-service/login":
			assert.Equal(t, "flow-1", r.URL.Query().Get("flow"))
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "password", body["method"])
			assert.Equal(t, "user@example.com", body["identifier"])
			assert.Equal(t, "secret", body["password"])
			json.NewEncoder(w).Encode(map[string]any{
				"session_token": "tok-1",
				"session": map[string]any{
					"identity": map[string]any{"traits": map[string]any{"email": "user@example.com"}},
				},
			})
		default:
			t.Errorf("неожиданный запрос: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	got, err := New(srv.URL, srv.Client()).Authenticate(context.Background(), "user@example.com", "secret")

	require.NoError(t, err)
	assert.Equal(t, "tok-1", got.Token)
	assert.Equal(t, "user@example.com", got.Email)
}

func TestAuthenticate_InvalidCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/self-service/login/api" {
			json.NewEncoder(w).Encode(map[string]any{"id": "flow-1"})
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).Authenticate(context.Background(), "user@example.com", "wrong")

	assert.ErrorIs(t, err, entities.ErrInvalidCredentials)
}

func TestInspect_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sessions/whoami", r.URL.Path)
		assert.Equal(t, "tok-1", r.Header.Get("X-Session-Token"))
		json.NewEncoder(w).Encode(map[string]any{
			"identity": map[string]any{"traits": map[string]any{"email": "user@example.com"}},
		})
	}))
	defer srv.Close()

	got, err := New(srv.URL, srv.Client()).Inspect(context.Background(), "tok-1")

	require.NoError(t, err)
	assert.Equal(t, "user@example.com", got.Email)
}

func TestInspect_ExpiredSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).Inspect(context.Background(), "tok-stale")

	assert.ErrorIs(t, err, entities.ErrSessionExpired)
}

func TestRevoke_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/self-service/logout/api", r.URL.Path)
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "tok-1", body["session_token"])
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	assert.NoError(t, New(srv.URL, srv.Client()).Revoke(context.Background(), "tok-1"))
}

func TestRevoke_AlreadyInvalidToken_NotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	assert.NoError(t, New(srv.URL, srv.Client()).Revoke(context.Background(), "tok-stale"))
}

func TestRevoke_ProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	assert.Error(t, New(srv.URL, srv.Client()).Revoke(context.Background(), "tok-1"))
}
