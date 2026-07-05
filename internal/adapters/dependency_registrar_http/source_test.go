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
	"github.com/TraumTech/paas-cli/internal/entities"
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

	dependency, err := src.RegisterDependency(context.Background(), svcID, verID, prodID, entities.ProtocolFormatOpenAPI, []byte(contract), nil, false)
	require.NoError(t, err)
	// Тело — обёртка {producer_service_id, document}: продьюсер и приложенный снимок.
	assert.Equal(t, prodID, gotBody["producer_service_id"])
	assert.Equal(t, "3.1.0", gotBody["document"].(map[string]interface{})["openapi"])
	assert.Equal(t, verID, dependency.ConsumerVersionID)
	assert.Equal(t, prodID, dependency.ProducerServiceID)
	_, hasSupersede := gotBody["supersede_previous"]
	assert.False(t, hasSupersede, "без замещения поле опускается")
}

func TestRegisterDependency_SupersedePreviousInBody(t *testing.T) {
	var gotBody map[string]interface{}
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"` + svcID + `","consumer_service_id":"` + svcID + `","consumer_version_id":"` + verID + `","producer_service_id":"` + prodID + `","format":"openapi","registered_at":"2026-06-15T00:00:00Z"}`))
	})

	_, err := src.RegisterDependency(context.Background(), svcID, verID, prodID, entities.ProtocolFormatOpenAPI, []byte(contract), nil, true)
	require.NoError(t, err)
	assert.Equal(t, true, gotBody["supersede_previous"])
}

func TestRegisterDependency_SendsMethods(t *testing.T) {
	var gotBody map[string]interface{}
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"` + svcID + `","consumer_service_id":"` + svcID + `","consumer_version_id":"` + verID + `","producer_service_id":"` + prodID + `","format":"openapi","methods":["GET /x"],"registered_at":"2026-06-15T00:00:00Z"}`))
	})

	_, err := src.RegisterDependency(context.Background(), svcID, verID, prodID, entities.ProtocolFormatOpenAPI, []byte(contract), []string{"GET /x", "POST /y"}, false)
	require.NoError(t, err)
	assert.Equal(t, []interface{}{"GET /x", "POST /y"}, gotBody["methods"])
}

func TestRegisterDependency_OmitsEmptyMethods(t *testing.T) {
	var gotBody map[string]interface{}
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"` + svcID + `","consumer_service_id":"` + svcID + `","consumer_version_id":"` + verID + `","producer_service_id":"` + prodID + `","format":"openapi","registered_at":"2026-06-15T00:00:00Z"}`))
	})

	_, err := src.RegisterDependency(context.Background(), svcID, verID, prodID, entities.ProtocolFormatOpenAPI, []byte(contract), nil, false)
	require.NoError(t, err)
	_, hasMethods := gotBody["methods"]
	assert.False(t, hasMethods, "пустой перечень методов опускается")
}

func TestRegisterDependency_NotFoundSurfacesDetail(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"title": "Not Found", "status": 404, "detail": "producer service not found"}`))
	})

	_, err := src.RegisterDependency(context.Background(), svcID, verID, prodID, entities.ProtocolFormatOpenAPI, []byte(contract), nil, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "producer service not found")
}

func TestRegisterDependency_InvalidServiceID(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("платформа не должна вызываться при неверном id")
	})

	_, err := src.RegisterDependency(context.Background(), "not-a-uuid", verID, prodID, entities.ProtocolFormatOpenAPI, []byte(contract), nil, false)
	require.Error(t, err)
}

func TestRegisterDependency_InvalidVersionID(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("платформа не должна вызываться при неверном id версии")
	})

	_, err := src.RegisterDependency(context.Background(), svcID, "not-a-uuid", prodID, entities.ProtocolFormatOpenAPI, []byte(contract), nil, false)
	require.Error(t, err)
}

func TestRegisterDependency_InvalidProducerID(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("платформа не должна вызываться при неверном id продьюсера")
	})

	_, err := src.RegisterDependency(context.Background(), svcID, verID, "not-a-uuid", entities.ProtocolFormatOpenAPI, []byte(contract), nil, false)
	require.Error(t, err)
}

func TestRegisterDependency_Unreachable(t *testing.T) {
	src, err := dependencyregistrarhttp.New("http://127.0.0.1:0", http.DefaultClient)
	require.NoError(t, err)
	_, err = src.RegisterDependency(context.Background(), svcID, verID, prodID, entities.ProtocolFormatOpenAPI, []byte(contract), nil, false)
	require.Error(t, err)
}

// gRPC-снимок уходит строкой с .proto и format=grpc; методы — gRPC-идентичностью.
func TestRegisterDependency_GRPCSnapshot(t *testing.T) {
	proto := "syntax = \"proto3\";\npackage traumtech.paas_protocols.v1;"
	var gotBody map[string]interface{}
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": "` + svcID + `", "consumer_service_id": "` + svcID + `", "consumer_version_id": "` + verID + `", "producer_service_id": "` + prodID + `", "format": "grpc", "registered_at": "2026-07-05T00:00:00Z"}`))
	})

	_, err := src.RegisterDependency(context.Background(), svcID, verID, prodID, entities.ProtocolFormatGRPC, []byte(proto),
		[]string{"traumtech.paas_protocols.v1.RegistryService/PublishProtocol"}, true)
	require.NoError(t, err)
	assert.Equal(t, "grpc", gotBody["format"])
	assert.Equal(t, proto, gotBody["document"], ".proto уходит строкой, не объектом")
	assert.Equal(t, []interface{}{"traumtech.paas_protocols.v1.RegistryService/PublishProtocol"}, gotBody["methods"])
	assert.Equal(t, true, gotBody["supersede_previous"])
}

// OpenAPI-регистрация не передаёт format — запрос не отличается от прежних.
func TestRegisterDependency_OpenAPIOmitsFormat(t *testing.T) {
	var gotBody map[string]interface{}
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": "` + svcID + `", "consumer_service_id": "` + svcID + `", "consumer_version_id": "` + verID + `", "producer_service_id": "` + prodID + `", "format": "openapi", "registered_at": "2026-06-15T00:00:00Z"}`))
	})

	_, err := src.RegisterDependency(context.Background(), svcID, verID, prodID, entities.ProtocolFormatOpenAPI, []byte(contract), nil, false)
	require.NoError(t, err)
	_, has := gotBody["format"]
	assert.False(t, has)
}

// Замена снимка — 200 (идемпотентный повтор при перезапуске выкатки) — успех.
func TestRegisterDependency_ReplacedOK(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "` + svcID + `", "consumer_service_id": "` + svcID + `", "consumer_version_id": "` + verID + `", "producer_service_id": "` + prodID + `", "format": "grpc", "registered_at": "2026-07-05T00:00:00Z"}`))
	})

	dependency, err := src.RegisterDependency(context.Background(), svcID, verID, prodID, entities.ProtocolFormatGRPC, []byte("syntax = \"proto3\";"), nil, false)
	require.NoError(t, err)
	assert.Equal(t, prodID, dependency.ProducerServiceID)
}
