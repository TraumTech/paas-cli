package dependencyregistrarhttp_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/adapters/dependency_registrar_http"
)

const (
	svcID  = "019ec073-3da6-705b-b19e-bbcca56656e1"
	verID  = "019ec073-3da6-705b-b19e-bbcca5665700"
	prodID = "019ec073-3da6-705b-b19e-bbcca5665711"
)

const contract = `{"openapi":"3.1.0","paths":{"/x":{}}}`

func newSource(t *testing.T, h http.HandlerFunc) *dependencyregistrarhttp.Source {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	src, err := dependencyregistrarhttp.New(srv.URL, srv.Client())
	require.NoError(t, err)
	return src
}

func TestRegisterDependency_SendsSnapshotAndMapsResult(t *testing.T) {
	var gotBody map[string]interface{}
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/services/"+svcID+"/versions/"+verID+"/dependencies", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{
			"id": "` + svcID + `",
			"consumer_service_id": "` + svcID + `",
			"consumer_version_id": "` + verID + `",
			"producer_service_id": "` + prodID + `",
			"format": "openapi",
			"registered_at": "2026-06-15T00:00:00Z"
		}`))
	})

	dependency, err := src.RegisterDependency(context.Background(), svcID, verID, prodID, []byte(contract))
	require.NoError(t, err)
	// Тело — обёртка {producer_service_id, document}: продьюсер и приложенный снимок.
	assert.Equal(t, prodID, gotBody["producer_service_id"])
	assert.Equal(t, "3.1.0", gotBody["document"].(map[string]interface{})["openapi"])
	assert.Equal(t, verID, dependency.ConsumerVersionID)
	assert.Equal(t, prodID, dependency.ProducerServiceID)
}

func TestRegisterDependency_NotFoundSurfacesDetail(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"title": "Not Found", "status": 404, "detail": "producer service not found"}`))
	})

	_, err := src.RegisterDependency(context.Background(), svcID, verID, prodID, []byte(contract))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "producer service not found")
}

func TestRegisterDependency_InvalidServiceID(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("платформа не должна вызываться при неверном id")
	})

	_, err := src.RegisterDependency(context.Background(), "not-a-uuid", verID, prodID, []byte(contract))
	require.Error(t, err)
}

func TestRegisterDependency_InvalidVersionID(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("платформа не должна вызываться при неверном id версии")
	})

	_, err := src.RegisterDependency(context.Background(), svcID, "not-a-uuid", prodID, []byte(contract))
	require.Error(t, err)
}

func TestRegisterDependency_InvalidProducerID(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("платформа не должна вызываться при неверном id продьюсера")
	})

	_, err := src.RegisterDependency(context.Background(), svcID, verID, "not-a-uuid", []byte(contract))
	require.Error(t, err)
}

func TestRegisterDependency_Unreachable(t *testing.T) {
	src, err := dependencyregistrarhttp.New("http://127.0.0.1:0", http.DefaultClient)
	require.NoError(t, err)
	_, err = src.RegisterDependency(context.Background(), svcID, verID, prodID, []byte(contract))
	require.Error(t, err)
}
