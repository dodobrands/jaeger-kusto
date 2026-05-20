package runner

import (
	"fmt"

	"github.com/dodopizza/jaeger-kusto/config"
	kustostore "github.com/dodopizza/jaeger-kusto/store"
	"github.com/hashicorp/go-hclog"
)

func servePlugin(_ *config.PluginConfig, _ *kustostore.Store, _ hclog.Logger) error {
	return fmt.Errorf("legacy Jaeger v1 plugin mode is no longer supported; set remoteMode=true to run the Jaeger v2 storage server")
}
