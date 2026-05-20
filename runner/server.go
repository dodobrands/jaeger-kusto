package runner

import (
	"fmt"
	"github.com/dodopizza/jaeger-kusto/config"
	storagev2 "github.com/dodopizza/jaeger-kusto/internal/proto/storage/v2"
	"github.com/dodopizza/jaeger-kusto/storagev2grpc"
	kustostore "github.com/dodopizza/jaeger-kusto/store"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

func serveServer(c *config.PluginConfig, store *kustostore.Store, logger hclog.Logger) error {
	tracer, closer, err := config.NewPluginTracer(c)
	if err != nil {
		return err
	}
	defer func() {
		if err := closer.Close(); err != nil {
			logger.Error("failed to close server tracer", "error", err)
		}
	}()

	server := newGRPCServerWithTracer(tracer)
	handler := storagev2grpc.NewHandler(store.SpanReader(), store.DependencyReader(), logger)
	handler.Register(server)

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(storagev2.TraceReader_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(storagev2.DependencyReader_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)

	scheme, address, err := parseListenAddress(c.RemoteListenAddress)
	if err != nil {
		return err
	}

	// perform cleanup for unix domain socket, before process exit
	if scheme == "unix" {
		defer func() {
			if err := os.Remove(address); err != nil && !os.IsNotExist(err) {
				logger.Warn("failed to remove unix socket", "error", err, "address", address)
			}
		}()
	}

	listener, err := net.Listen(scheme, address)
	if err != nil {
		return err
	}

	logger.Info("starting server", "address", address, "scheme", scheme)
	wg := registerGracefulShutdown(server, logger)
	if err := server.Serve(listener); err != nil {
		return err
	}

	wg.Wait()
	return nil
}

func registerGracefulShutdown(server *grpc.Server, logger hclog.Logger) *sync.WaitGroup {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		sig := <-signals
		logger.Info("received signal, attempting gracefully stop server", "signal", sig)
		server.GracefulStop()
		logger.Info("server stopped")
		wg.Done()
	}()

	return wg
}

func parseListenAddress(addr string) (scheme, address string, err error) {
	u, err := url.Parse(addr)
	if err != nil {
		return "", "", err
	}

	proto := fmt.Sprintf("%s://", u.Scheme)

	return u.Scheme, strings.Replace(addr, proto, "", 1), nil
}
