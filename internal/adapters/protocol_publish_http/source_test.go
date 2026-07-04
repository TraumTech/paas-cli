package protocolpublishhttp_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/adapters/protocol_publish_http"
	"github.com/TraumTech/paas-cli/internal/entities"
)

const (
	svcID = "019ec073-3da6-705b-b19e-bbcca56656e1"
	verID = "019ec073-3da6-705b-b19e-bbcca5665700"
)

func newSource(t *testing.T, h http.HandlerFunc) *protocolpublishhttp.Source {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	src, err := protocolpublishhttp.New(srv.URL, srv.Client())
	require.NoError(t, err)
	return src
}

// gRPC-контракт уходит с параметром формата, тело — JSON-строка с .proto-исходником.
func TestPublishProtocol_GRPCFormatAndBody(t *testing.T) {
	proto := "syntax = \"proto3\";\npackage traumtech.paas_protocols.v1;"
	var gotFormat, gotBody string
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		gotFormat = r.URL.Query().Get("format")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"published": true, "breaking": false, "protocol": {"service_id": "` + svcID + `", "version_id": "` + verID + `", "version_number": 1, "format": "grpc", "published_at": "2026-07-03T00:00:00Z"}, "consumers": []}`))
	})

	publication, err := src.PublishProtocol(context.Background(), svcID, verID, entities.ProtocolFormatGRPC, []byte(proto))
	require.NoError(t, err)
	assert.Equal(t, "grpc", gotFormat)
	expected, _ := json.Marshal(proto)
	assert.Equal(t, string(expected), gotBody)
	assert.Equal(t, 1, publication.VersionNumber)
}

// Прежний формат не передаётся в запросе — публикация без типа, как раньше.
func TestPublishProtocol_OpenAPIOmitsFormat(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		assert.False(t, r.URL.Query().Has("format"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"published": true, "breaking": false, "protocol": {"service_id": "` + svcID + `", "version_id": "` + verID + `", "version_number": 7, "format": "openapi", "published_at": "2026-06-15T00:00:00Z"}, "consumers": []}`))
	})

	_, err := src.PublishProtocol(context.Background(), svcID, verID, entities.ProtocolFormatOpenAPI, []byte(`{"openapi":"3.1.0"}`))
	require.NoError(t, err)
}

func TestPublishProtocol_MapsPublication(t *testing.T) {
	var gotBody string
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/services/"+svcID+"/versions/"+verID+"/protocol", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{
			"published": true,
			"breaking": true,
			"protocol": {"service_id": "` + svcID + `", "version_id": "` + verID + `", "version_number": 7, "format": "openapi", "published_at": "2026-06-15T00:00:00Z"},
			"consumers": [{
				"consumer_service_id": "` + svcID + `",
				"consumer_service_name": "frontend",
				"consumer_version_id": "` + verID + `",
				"consumer_version_number": 5,
				"comparable": true,
				"breaking": true,
				"changes": [
					{"breaking": true, "kind": "operation-removed", "operation": "GET /x", "description": "удалён эндпоинт"}
				]
			}]
		}`))
	})

	publication, err := src.PublishProtocol(context.Background(), svcID, verID, entities.ProtocolFormatOpenAPI, []byte(`{"openapi":"3.1.0"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"openapi":"3.1.0"}`, gotBody)
	assert.Equal(t, 7, publication.VersionNumber)
	assert.True(t, publication.Breaking)
	require.Len(t, publication.Consumers, 1)
	c := publication.Consumers[0]
	assert.Equal(t, "frontend", c.ServiceName)
	assert.Equal(t, 5, c.VersionNumber)
	assert.True(t, c.Breaking)
	require.Len(t, c.Changes, 1)
	assert.Equal(t, "operation-removed", c.Changes[0].Kind)
	assert.Equal(t, "GET /x", c.Changes[0].Operation)
	assert.Equal(t, "удалён эндпоинт", c.Changes[0].Description)
}

func TestPublishProtocol_NoConsumers(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"published": true, "breaking": false, "protocol": {"service_id": "` + svcID + `", "version_id": "` + verID + `", "version_number": 7, "format": "openapi", "published_at": "2026-06-15T00:00:00Z"}, "consumers": []}`))
	})

	publication, err := src.PublishProtocol(context.Background(), svcID, verID, entities.ProtocolFormatOpenAPI, []byte(`{"openapi":"3.1.0"}`))
	require.NoError(t, err)
	assert.Equal(t, 7, publication.VersionNumber)
	assert.False(t, publication.Breaking)
	assert.Empty(t, publication.Consumers)
}

// Повторная публикация для версии, у которой протокол уже был, отвечает 200 —
// идемпотентный повтор при перезапуске выкатки успешен, как и первая публикация.
func TestPublishProtocol_RepublishOK(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"published": true, "breaking": false, "protocol": {"service_id": "` + svcID + `", "version_id": "` + verID + `", "version_number": 7, "format": "openapi", "published_at": "2026-06-15T00:00:00Z"}, "consumers": []}`))
	})

	publication, err := src.PublishProtocol(context.Background(), svcID, verID, entities.ProtocolFormatOpenAPI, []byte(`{"openapi":"3.1.0"}`))
	require.NoError(t, err)
	assert.Equal(t, 7, publication.VersionNumber)
}

func TestPublishProtocol_NotFoundSurfacesDetail(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"title": "Not Found", "status": 404, "detail": "version not found"}`))
	})

	_, err := src.PublishProtocol(context.Background(), svcID, verID, entities.ProtocolFormatOpenAPI, []byte(`{"openapi":"3.1.0"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version not found")
}

func TestPublishProtocol_InvalidServiceID(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("платформа не должна вызываться при неверном id")
	})

	_, err := src.PublishProtocol(context.Background(), "not-a-uuid", verID, entities.ProtocolFormatOpenAPI, []byte(`{"openapi":"3.1.0"}`))
	require.Error(t, err)
}

func TestPublishProtocol_InvalidVersionID(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("платформа не должна вызываться при неверном id версии")
	})

	_, err := src.PublishProtocol(context.Background(), svcID, "not-a-uuid", entities.ProtocolFormatOpenAPI, []byte(`{"openapi":"3.1.0"}`))
	require.Error(t, err)
}

func TestPublishProtocol_Unreachable(t *testing.T) {
	src, err := protocolpublishhttp.New("http://127.0.0.1:0", http.DefaultClient)
	require.NoError(t, err)
	_, err = src.PublishProtocol(context.Background(), svcID, verID, entities.ProtocolFormatOpenAPI, []byte(`{"openapi":"3.1.0"}`))
	require.Error(t, err)
}
