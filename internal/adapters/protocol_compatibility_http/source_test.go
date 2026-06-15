package protocolcompatibilityhttp_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/adapters/protocol_compatibility_http"
	"github.com/TraumTech/paas-cli/internal/entities"
)

const svcID = "019ec073-3da6-705b-b19e-bbcca56656e1"

func newSource(t *testing.T, h http.HandlerFunc) *protocolcompatibilityhttp.Source {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	src, err := protocolcompatibilityhttp.New(srv.URL, srv.Client())
	require.NoError(t, err)
	return src
}

func TestCheckCompatibility_MapsReport(t *testing.T) {
	var gotBody string
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/services/"+svcID+"/protocol/compatibility", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"breaking": true,
			"consumers": [{
				"consumer_service_id": "` + svcID + `",
				"consumer_service_name": "frontend",
				"consumer_version_id": "` + svcID + `",
				"consumer_version_number": 5,
				"comparable": true,
				"breaking": true,
				"changes": [
					{"breaking": true, "kind": "operation-removed", "operation": "GET /x", "description": "удалён эндпоинт"}
				]
			}]
		}`))
	})

	report, err := src.CheckCompatibility(context.Background(), svcID, []byte(`{"openapi":"3.1.0"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"openapi":"3.1.0"}`, gotBody)
	assert.True(t, report.Breaking)
	require.Len(t, report.Consumers, 1)
	c := report.Consumers[0]
	assert.Equal(t, "frontend", c.ServiceName)
	assert.Equal(t, 5, c.VersionNumber)
	assert.True(t, c.Comparable)
	assert.True(t, c.Breaking)
	require.Len(t, c.Changes, 1)
	assert.Equal(t, "operation-removed", c.Changes[0].Kind)
	assert.Equal(t, "GET /x", c.Changes[0].Operation)
	assert.Equal(t, "удалён эндпоинт", c.Changes[0].Description)
}

func TestCheckCompatibility_NoConsumers(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"breaking": false, "consumers": []}`))
	})

	report, err := src.CheckCompatibility(context.Background(), svcID, []byte(`{"openapi":"3.1.0"}`))
	require.NoError(t, err)
	assert.False(t, report.Breaking)
	assert.Empty(t, report.Consumers)
}

func TestCheckCompatibility_ServiceNotFound(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := src.CheckCompatibility(context.Background(), svcID, []byte(`{"openapi":"3.1.0"}`))
	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
}

func TestCheckCompatibility_InvalidCandidate(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	_, err := src.CheckCompatibility(context.Background(), svcID, []byte(`{}`))
	assert.ErrorIs(t, err, entities.ErrInvalidProtocol)
}

func TestCheckCompatibility_InvalidID(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("платформа не должна вызываться при неверном id")
	})

	_, err := src.CheckCompatibility(context.Background(), "not-a-uuid", []byte(`{"openapi":"3.1.0"}`))
	require.Error(t, err)
}

func TestCheckCompatibility_Unreachable(t *testing.T) {
	src, err := protocolcompatibilityhttp.New("http://127.0.0.1:0", http.DefaultClient)
	require.NoError(t, err)
	_, err = src.CheckCompatibility(context.Background(), svcID, []byte(`{"openapi":"3.1.0"}`))
	require.Error(t, err)
}
