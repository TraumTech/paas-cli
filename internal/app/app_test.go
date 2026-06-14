package app_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/app"
)

func fakePlatform(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/services/svc-1":
			w.Write([]byte(`{"id":"svc-1","name":"payments"}`))
		case "/services/svc-1/protocol":
			w.Write([]byte(`{"published":true,"version_number":2,"format":"openapi","document":{"openapi":"3.1.0","paths":{"/x":{}}}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestRun_FetchWritesContract — сквозной тест сборки: PAAS_API_URL указывает на
// фейковую платформу, команда тянет протокол и пишет его в <dest>/<name>/openapi.json.
func TestRun_FetchWritesContract(t *testing.T) {
	srv := fakePlatform(t)
	defer srv.Close()

	t.Setenv("PAAS_API_URL", srv.URL)
	dest := t.TempDir()

	err := app.Run(context.Background(),
		[]string{"paas-cli", "--destination", dest, "protocols", "fetch", "svc-1"})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dest, "payments", "openapi.json"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"openapi":"3.1.0","paths":{"/x":{}}}`, string(data))
}

func TestRun_FetchNotPublished(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/services/svc-1" {
			w.Write([]byte(`{"name":"payments"}`))
			return
		}
		w.Write([]byte(`{"published":false}`))
	}))
	defer srv.Close()

	t.Setenv("PAAS_API_URL", srv.URL)
	err := app.Run(context.Background(),
		[]string{"paas-cli", "--destination", t.TempDir(), "protocols", "fetch", "svc-1"})
	require.Error(t, err)
}
