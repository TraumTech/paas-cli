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

const svcID = "019ec073-3da6-705b-b19e-bbcca56656e1"

func newSource(t *testing.T, h http.HandlerFunc) *protocolsourcehttp.Source {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	src, err := protocolsourcehttp.New(srv.URL, srv.Client())
	require.NoError(t, err)
	return src
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(body))
}

func TestFetchProtocol_Published(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/services/" + svcID:
			writeJSON(w, `{"id":"`+svcID+`","name":"payments"}`)
		case "/services/" + svcID + "/protocol":
			writeJSON(w, `{"published":true,"version_number":4,"format":"openapi","document":{"openapi":"3.1.0","paths":{}}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	got, err := src.FetchProtocol(context.Background(), svcID)
	require.NoError(t, err)
	assert.Equal(t, svcID, got.ServiceID)
	assert.Equal(t, "payments", got.ServiceName)
	assert.Equal(t, 4, got.VersionNumber)
	assert.Equal(t, "openapi", got.Format)
	assert.JSONEq(t, `{"openapi":"3.1.0","paths":{}}`, string(got.Document))
}

func TestFetchProtocol_NotPublished(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/services/"+svcID {
			writeJSON(w, `{"id":"`+svcID+`","name":"payments"}`)
			return
		}
		writeJSON(w, `{"published":false}`)
	})

	_, err := src.FetchProtocol(context.Background(), svcID)
	assert.ErrorIs(t, err, entities.ErrProtocolNotPublished)
}

func TestFetchProtocol_ServiceNotFound(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := src.FetchProtocol(context.Background(), svcID)
	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
}

func TestFetchProtocol_ServerError(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := src.FetchProtocol(context.Background(), svcID)
	require.Error(t, err)
	assert.NotErrorIs(t, err, entities.ErrServiceNotFound)
}

func TestFetchProtocol_InvalidID(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("платформа не должна вызываться при неверном id")
	})

	_, err := src.FetchProtocol(context.Background(), "not-a-uuid")
	require.Error(t, err)
}

func TestFetchProtocol_Unreachable(t *testing.T) {
	src, err := protocolsourcehttp.New("http://127.0.0.1:0", http.DefaultClient)
	require.NoError(t, err)
	_, err = src.FetchProtocol(context.Background(), svcID)
	require.Error(t, err)
}
