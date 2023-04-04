package components

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

type pprofServer struct {
	server *http.Server
}

func newPPROFServer(pprofAddr string) *pprofServer {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	s := &http.Server{
		Addr:              pprofAddr,
		Handler:           mux,
		ReadHeaderTimeout: 1 * time.Second,
	}

	return &pprofServer{
		server: s,
	}
}

func (s *pprofServer) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		s.server.Close()
	}()
	return s.server.ListenAndServe()
}

func registerPPROF(mgr ctrl.Manager, pprofAddr string) error {
	if len(pprofAddr) == 0 {
		return nil
	}

	s := newPPROFServer(pprofAddr)
	err := mgr.Add(s)
	if err != nil {
		return fmt.Errorf("unable to register pprof server: %w", err)
	}
	return nil
}
