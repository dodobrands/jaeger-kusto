package runner

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServePlugin_ReturnsHelpfulError(t *testing.T) {
	err := servePlugin(nil, nil, nil)

	require.EqualError(t, err, "legacy Jaeger v1 plugin mode is no longer supported; set remoteMode=true to run the Jaeger v2 storage server")
}
