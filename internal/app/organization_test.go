package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithOrganization(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Organization-Id")
	}))
	defer srv.Close()

	t.Run("выбранная организация уходит заголовком", func(t *testing.T) {
		client := withOrganization(&http.Client{Transport: http.DefaultTransport}, "org-2")
		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, "org-2", got)
	})

	t.Run("без выбора заголовка нет — платформа берёт личную", func(t *testing.T) {
		client := withOrganization(&http.Client{Transport: http.DefaultTransport}, "")
		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Empty(t, got)
	})
}
