package app_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/app"
)

const svcID = "019ec073-3da6-705b-b19e-bbcca56656e1"

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(body))
}

func fakePlatform(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/services/" + svcID:
			writeJSON(w, `{"id":"`+svcID+`","name":"payments"}`)
		case "/services/" + svcID + "/protocol":
			writeJSON(w, `{"published":true,"version_number":2,"format":"openapi","document":{"openapi":"3.1.0","paths":{"/x":{}}}}`)
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
		[]string{"paas-cli", "--destination", dest, "protocols", "fetch", svcID})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dest, "payments", "openapi.json"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"openapi":"3.1.0","paths":{"/x":{}}}`, string(data))
}

// TestRun_CompatibilityBreaking — сквозной тест owner-команды: PAAS_API_URL
// указывает на фейковую платформу, команда шлёт кандидат на проверку и при
// ломающем вердикте завершается ошибкой (ненулевой код останавливает выкатку).
func TestRun_CompatibilityBreaking(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/services/"+svcID+"/protocol/compatibility", r.URL.Path)
		writeJSON(w, `{"breaking":true,"consumers":[{"consumer_service_id":"`+svcID+`","consumer_service_name":"frontend","consumer_version_id":"`+svcID+`","consumer_version_number":5,"comparable":true,"breaking":true,"changes":[{"breaking":true,"kind":"operation-removed","operation":"GET /x","description":"удалён"}]}]}`)
	}))
	defer srv.Close()

	candidate := filepath.Join(t.TempDir(), "openapi.json")
	require.NoError(t, os.WriteFile(candidate, []byte(`{"openapi":"3.1.0","paths":{"/x":{}}}`), 0o644))

	t.Setenv("PAAS_API_URL", srv.URL)
	err := app.Run(context.Background(),
		[]string{"paas-cli", "protocols", "compatibility", svcID, candidate})
	require.Error(t, err)
}

func TestRun_CompatibilityNoConsumers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"breaking":false,"consumers":[]}`)
	}))
	defer srv.Close()

	candidate := filepath.Join(t.TempDir(), "openapi.json")
	require.NoError(t, os.WriteFile(candidate, []byte(`{"openapi":"3.1.0","paths":{"/x":{}}}`), 0o644))

	t.Setenv("PAAS_API_URL", srv.URL)
	require.NoError(t, app.Run(context.Background(),
		[]string{"paas-cli", "protocols", "compatibility", svcID, candidate}))
}

// TestRun_PublishVersionIdempotent — сквозной тест owner-команды: фейковая
// платформа идемпотентна (одна ревизия — одна версия), и оба прогона CLI печатают
// на stdout один и тот же id версии, пригодный для следующего шага выкатки.
func TestRun_PublishVersionIdempotent(t *testing.T) {
	const versionID = "019ec099-0000-7000-8000-0000000000aa"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/services/"+svcID+"/versions", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, `{"id":"`+versionID+`","number":3,"commit_revision":"deadbeef","created_at":"2026-06-15T10:00:00Z"}`)
	}))
	defer srv.Close()

	t.Setenv("PAAS_API_URL", srv.URL)
	for range 2 {
		stdout := captureStdout(t, func() {
			require.NoError(t, app.Run(context.Background(),
				[]string{"paas-cli", "versions", "publish", svcID, "deadbeef"}))
		})
		assert.Equal(t, versionID, strings.TrimSpace(stdout))
	}
}

// captureStdout подменяет os.Stdout на время вызова fn и возвращает написанное:
// команда публикации печатает id версии прямо на stdout (контракт для автоматики),
// а собственного writer'а app.Run наружу не отдаёт.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	require.NoError(t, w.Close())
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(data)
}

func TestRun_FetchNotPublished(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/services/"+svcID {
			writeJSON(w, `{"id":"`+svcID+`","name":"payments"}`)
			return
		}
		writeJSON(w, `{"published":false}`)
	}))
	defer srv.Close()

	t.Setenv("PAAS_API_URL", srv.URL)
	err := app.Run(context.Background(),
		[]string{"paas-cli", "--destination", t.TempDir(), "protocols", "fetch", svcID})
	require.Error(t, err)
}
