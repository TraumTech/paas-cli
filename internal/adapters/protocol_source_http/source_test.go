package protocolsourcehttp_test

import (
	"context"
	"encoding/json"
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
	assert.Equal(t, entities.ProtocolFormatOpenAPI, got.Format)
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

// gRPC-протокол приходит с document_text (.proto-исходник) и явным форматом.
func TestFetchProtocol_GRPCFromDocumentText(t *testing.T) {
	proto := "syntax = \"proto3\";\npackage traumtech.paas_protocols.v1;"
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/services/" + svcID:
			writeJSON(w, `{"id":"`+svcID+`","name":"paas-protocols"}`)
		case "/services/" + svcID + "/protocol":
			body, _ := json.Marshal(map[string]any{
				"published": true, "version_number": 1, "format": "grpc", "document_text": proto,
			})
			writeJSON(w, string(body))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	got, err := src.FetchProtocol(context.Background(), svcID)
	require.NoError(t, err)
	assert.Equal(t, entities.ProtocolFormatGRPC, got.Format)
	assert.Equal(t, proto, string(got.Document))
	assert.Equal(t, 1, got.VersionNumber)
}

// Формат, которого CLI не знает, — честная ошибка, а не контракт, разложенный
// как попало.
func TestFetchProtocol_UnknownFormat(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/services/" + svcID:
			writeJSON(w, `{"id":"`+svcID+`","name":"payments"}`)
		case "/services/" + svcID + "/protocol":
			writeJSON(w, `{"published":true,"version_number":2,"format":"graphql","document_text":"whatever"}`)
		}
	})

	_, err := src.FetchProtocol(context.Background(), svcID)
	var unsupported *entities.UnsupportedProtocolFormatError
	require.ErrorAs(t, err, &unsupported)
	assert.Equal(t, "graphql", unsupported.Name)
}
