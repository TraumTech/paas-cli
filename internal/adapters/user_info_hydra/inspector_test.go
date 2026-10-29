package userinfohydra

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

func TestInspect_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/userinfo", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"sub": "u-1", "email": "user@example.com"})
	}))
	defer srv.Close()

	got, err := New(srv.URL, srv.Client()).Inspect(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "user@example.com", got.Email)
	assert.Equal(t, "u-1", got.UserID)
}

func TestInspect_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).Inspect(context.Background())

	assert.ErrorIs(t, err, entities.ErrSessionExpired)
}
