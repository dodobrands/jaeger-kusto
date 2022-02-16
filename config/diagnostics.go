package config

import (
	"github.com/hashicorp/go-hclog"
	"net"
	"net/http"
	"net/http/pprof"
)

func ServeDiagnosticsServer(pc *PluginConfig, logger hclog.Logger) error {
	listener, err := net.Listen("tcp", pc.DiagnosticsListenAddress)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", live)

	if pc.DiagnosticsProfilingEnabled {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
		mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
		mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	}

	server := http.Server{
		Handler: mux,
	}

	go func() {
		logger.Info("starting diagnostics server at address", "address", pc.DiagnosticsListenAddress)
		_ = server.Serve(listener)
	}()

	return nil
}

func live(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
