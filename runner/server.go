package runner

import (
	"github.com/dodopizza/jaeger-kusto/config"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"
	"google.golang.org/grpc"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func serveServer(c *config.PluginConfig, store shared.StoragePlugin, logger hclog.Logger) error {
	plugin := shared.StorageGRPCPlugin{
		Impl: store,
	}

	tracer, closer, err := config.NewPluginTracer(c)
	if err != nil {
		return err
	}
	defer closer.Close()

	server := newGRPCServerWithTracer(tracer)
	if err := plugin.GRPCServer(nil, server); err != nil {
		return err
	}

	listener, err := net.Listen("tcp", c.RemoteAddress)
	if err != nil {
		return err
	}

	logger.Info("starting server on address", "address", listener.Addr())
	wg := registerGracefulShutdown(server, store, logger)
	if err := server.Serve(listener); err != nil {
		return err
	}

	wg.Wait()
	return nil
}

func registerGracefulShutdown(server *grpc.Server, store shared.StoragePlugin, logger hclog.Logger) *sync.WaitGroup {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		sig := <-signals
		logger.Info("received signal, attempting gracefully stop server and plugin", "signal", sig)
		server.GracefulStop()

		// perform cleanup logic on writer
		c, ok := store.SpanWriter().(io.Closer)
		if ok {
			_ = c.Close()
		}

		logger.Info("server stopped")
		wg.Done()
	}()

	return wg
}
