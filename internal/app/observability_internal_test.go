package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTracesURL(t *testing.T) {
	require.Equal(t, "https://otlp.paas.traumtech.ru/v1/traces", tracesURL("https://otlp.paas.traumtech.ru"))
	require.Equal(t, "https://otlp.paas.traumtech.ru/v1/traces", tracesURL("https://otlp.paas.traumtech.ru/"))
	require.Equal(t, "http://localhost:4318/custom", tracesURL("http://localhost:4318/custom"))
}
