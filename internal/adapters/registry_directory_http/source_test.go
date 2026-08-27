package registrydirectoryhttp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/adapters/registry_directory_http"
	"github.com/TraumTech/paas-cli/internal/entities"
)

const svcID = "019ec073-3da6-705b-b19e-bbcca56656e1"

func newSource(t *testing.T, h http.HandlerFunc) *registrydirectoryhttp.Source {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	src, err := registrydirectoryhttp.New(srv.URL, srv.Client())
	require.NoError(t, err)
	return src
}

func TestListProtocols(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/services/"+svcID+"/protocols", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"service_id":"` + svcID + `","protocols":[
			{"service_id":"` + svcID + `","version_id":"` + svcID + `","version_number":7,"name":"default","format":"openapi","published_at":"2026-08-27T00:00:00Z"},
			{"service_id":"` + svcID + `","version_id":"` + svcID + `","version_number":7,"name":"admin","format":"openapi","published_at":"2026-08-27T00:00:00Z"}
		]}`))
	})

	got, err := src.ListProtocols(context.Background(), svcID)
	require.NoError(t, err)
	assert.Equal(t, []entities.RegistryProtocol{
		{Name: "default", Format: "openapi"},
		{Name: "admin", Format: "openapi"},
	}, got)
}

func TestListConsumers(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/services/"+svcID+"/consumers", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{
			"consumer_service_id":"` + svcID + `",
			"consumer_service_name":"paas-frontend",
			"consumer_version_id":"` + svcID + `",
			"consumer_version_number":12,
			"format":"openapi",
			"methods":["GET /services"],
			"waived_attributes":[],
			"registered_at":"2026-08-27T00:00:00Z"
		}]`))
	})

	got, err := src.ListConsumers(context.Background(), svcID)
	require.NoError(t, err)
	assert.Equal(t, []entities.RegisteredConsumer{{ServiceName: "paas-frontend", VersionNumber: 12}}, got)
}

func TestListProtocols_NotFound(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"title":"Not Found"}`))
	})

	_, err := src.ListProtocols(context.Background(), svcID)
	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
}
