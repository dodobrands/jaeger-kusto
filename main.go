package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/dodopizza/jaeger-kusto/metrics"
	"github.com/dodopizza/jaeger-kusto/runner"

	"github.com/dodopizza/jaeger-kusto/config"
	"github.com/dodopizza/jaeger-kusto/store"
)

func main() {
	configPath := ""
	flag.StringVar(&configPath, "config", "", "The path to the plugin's configuration file")
	flag.Parse()

	pluginConfig, err := config.ParseConfig(configPath)
	if err != nil {
		os.Exit(1)
	}

	logger := config.NewLogger(pluginConfig)
	logger.Info("plugin config", "config", pluginConfig)

	if err := config.ServeDiagnosticsServer(pluginConfig, logger); err != nil {
		logger.Error("error occurred while starting diagnostics server", "error", err)
		os.Exit(1)
	}

	kustoConfig, err := config.ParseKustoConfig(pluginConfig.KustoConfigPath, pluginConfig.ReadNoTruncation, pluginConfig.ReadNoTimeout)
	if err != nil {
		logger.Error("error occurred while reading kusto configuration", "error", err)
		os.Exit(1)
	}

	kustoStore, err := store.NewStore(kustoConfig, pluginConfig, logger)
	if err != nil {
		logger.Error("error occurred while initializing kusto storage", "error", err)
		os.Exit(2)
	}

	// Start the PromQL metrics shim server if enabled
	if pluginConfig.MetricsEnabled {
		kustoClient, err := store.NewKustoClient(kustoConfig, logger)
		if err != nil {
			logger.Error("error occurred while creating kusto client for metrics", "error", err)
			os.Exit(2)
		}
		metricsReader := metrics.NewKustoMetricsReader(metrics.KustoMetricsReaderConfig{
			Client:      kustoClient,
			Database:    kustoConfig.Database,
			TraceTable:  kustoConfig.TraceTableName,
			MetricsView: kustoConfig.MetricsViewName,
			Logger:      logger,
			ReadOptions: kustoConfig.ClientRequestOptions,
		})
		metricsServer := metrics.NewServer(metrics.ServerConfig{
			Reader: metricsReader,
			Logger: logger,
		})
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := metricsServer.Shutdown(ctx); err != nil {
				logger.Error("failed to gracefully shutdown metrics server", "error", err)
			}
		}()
		go func() {
			if err := metricsServer.ListenAndServe(pluginConfig.MetricsListenAddress); err != nil {
				if errors.Is(err, http.ErrServerClosed) {
					return
				}
				logger.Error("metrics server error", "error", err)
				os.Exit(2)
			}
		}()
	}

	if err := runner.Serve(pluginConfig, kustoStore, logger); err != nil {
		logger.Error("error occurred while invoking runner", "error", err)
		os.Exit(3)
	}
}
