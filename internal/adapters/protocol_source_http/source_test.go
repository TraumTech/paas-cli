package protocolsourcehttp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/adapters/protocol_source_http"
	"github.com/TraumTech/paas-cli/internal/entities"
)

func newSource(h http.HandlerFunc) (*protocolsourcehttp.Source, func()) {
	srv := httptest.NewServer(h)
	return protocolsourcehttp.New(srv.URL, srv.Client()), srv.Close
}

func TestFetchProtocol_Published(t *testing.T) {
	src, closeFn := newSource(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/services/svc-1":
			w.Write([]byte(`{"id":"svc-1","name":"payments"}`))
		case "/services/svc-1/protocol":
			w.Write([]byte(`{"published":true,"version_number":4,"format":"openapi","document":{"openapi":"3.1.0","paths":{}}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})
	defer closeFn()

	got, err := src.FetchProtocol(context.Background(), "svc-1")
	require.NoError(t, err)
	assert.Equal(t, "svc-1", got.ServiceID)
	assert.Equal(t, "payments", got.ServiceName)
	assert.Equal(t, 4, got.VersionNumber)
	assert.Equal(t, "openapi", got.Format)
	assert.JSONEq(t, `{"openapi":"3.1.0","paths":{}}`, string(got.Document))
}

func TestFetchProtocol_NotPublished(t *testing.T) {
	src, closeFn := newSource(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/services/svc-1":
			w.Write([]byte(`{"name":"payments"}`))
		default:
			w.Write([]byte(`{"published":false}`))
		}
	})
	defer closeFn()

	_, err := src.FetchProtocol(context.Background(), "svc-1")
	assert.ErrorIs(t, err, entities.ErrProtocolNotPublished)
}

func TestFetchProtocol_ServiceNotFound(t *testing.T) {
	src, closeFn := newSource(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer closeFn()

	_, err := src.FetchProtocol(context.Background(), "missing")
	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
}

func TestFetchProtocol_ServerError(t *testing.T) {
	src, closeFn := newSource(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer closeFn()

	_, err := src.FetchProtocol(context.Background(), "svc-1")
	require.Error(t, err)
	assert.NotErrorIs(t, err, entities.ErrServiceNotFound)
}

func TestFetchProtocol_Unreachable(t *testing.T) {
	src := protocolsourcehttp.New("http://127.0.0.1:0", http.DefaultClient)
	_, err := src.FetchProtocol(context.Background(), "svc-1")
	require.Error(t, err)
}
