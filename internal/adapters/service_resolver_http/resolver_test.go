package serviceresolverhttp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/adapters/service_resolver_http"
)

const backendID = "019ec073-3da6-705b-b19e-bbcca56656e1"

func newResolver(t *testing.T, h http.HandlerFunc) *serviceresolverhttp.Resolver {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	r, err := serviceresolverhttp.New(srv.URL, srv.Client())
	require.NoError(t, err)
	return r
}

func servicesList(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`[{"id":"` + backendID + `","name":"paas-backend","created_at":"2026-01-01T00:00:00Z"},` +
		`{"id":"019ec073-0000-705b-b19e-000000000000","name":"billing","created_at":"2026-01-01T00:00:00Z"}]`))
}

func TestResolveIDs_MapsFoundNames(t *testing.T) {
	r := newResolver(t, func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "/services", req.URL.Path)
		// фильтр передаётся серверу повторяемым параметром name
		assert.ElementsMatch(t, []string{"paas-backend", "billing"}, req.URL.Query()["name"])
		servicesList(w)
	})

	ids, err := r.ResolveIDs(context.Background(), []string{"paas-backend", "billing"})
	require.NoError(t, err)
	assert.Equal(t, backendID, ids["paas-backend"])
	assert.Equal(t, "019ec073-0000-705b-b19e-000000000000", ids["billing"])
}

func TestResolveIDs_MissingNameAbsentFromMap(t *testing.T) {
	r := newResolver(t, func(w http.ResponseWriter, _ *http.Request) { servicesList(w) })

	ids, err := r.ResolveIDs(context.Background(), []string{"ghost"})
	require.NoError(t, err)
	_, ok := ids["ghost"]
	assert.False(t, ok)
}

func TestResolveIDs_ServerError(t *testing.T) {
	r := newResolver(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := r.ResolveIDs(context.Background(), []string{"paas-backend"})
	require.Error(t, err)
}

func TestResolveIDs_Unreachable(t *testing.T) {
	r, err := serviceresolverhttp.New("http://127.0.0.1:0", http.DefaultClient)
	require.NoError(t, err)
	_, err = r.ResolveIDs(context.Background(), []string{"paas-backend"})
	require.Error(t, err)
}
