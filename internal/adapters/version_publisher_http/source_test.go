package versionpublisherhttp_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/adapters/version_publisher_http"
	"github.com/TraumTech/paas-cli/internal/entities"
)

const svcID = "019ec073-3da6-705b-b19e-bbcca56656e1"

func newSource(t *testing.T, h http.HandlerFunc) *versionpublisherhttp.Source {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	src, err := versionpublisherhttp.New(srv.URL, srv.Client())
	require.NoError(t, err)
	return src
}

func TestPublishVersion_MapsResponse(t *testing.T) {
	var gotBody map[string]any
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/services/"+svcID+"/versions", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"` + svcID + `","number":7,"commit_revision":"abc123","created_at":"2026-06-15T10:00:00Z"}`))
	})

	version, err := src.PublishVersion(context.Background(), svcID, "abc123")
	require.NoError(t, err)
	assert.Equal(t, "abc123", gotBody["commit_revision"])
	assert.Equal(t, svcID, version.ID)
	assert.Equal(t, 7, version.Number)
	assert.Equal(t, "abc123", version.CommitRevision)
}

func TestPublishVersion_ServiceNotFound(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := src.PublishVersion(context.Background(), svcID, "abc123")
	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
}

func TestPublishVersion_UnexpectedStatus(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := src.PublishVersion(context.Background(), svcID, "abc123")
	require.Error(t, err)
	assert.NotErrorIs(t, err, entities.ErrServiceNotFound)
}

func TestPublishVersion_InvalidID(t *testing.T) {
	src := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("платформа не должна вызываться при неверном id")
	})

	_, err := src.PublishVersion(context.Background(), "not-a-uuid", "abc123")
	require.Error(t, err)
}

func TestPublishVersion_Unreachable(t *testing.T) {
	src, err := versionpublisherhttp.New("http://127.0.0.1:0", http.DefaultClient)
	require.NoError(t, err)
	_, err = src.PublishVersion(context.Background(), svcID, "abc123")
	require.Error(t, err)
}
