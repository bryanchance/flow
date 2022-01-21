package profiler

import (
	"net/http"
	"net/http/pprof"
)

type Profiler struct {
	addr string
	mux  *http.ServeMux
}

func NewProfiler(addr string) *Profiler {
	mux := http.NewServeMux()

	// Add the pprof routes
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))

	return &Profiler{
		addr: addr,
		mux:  mux,
	}
}

func (p *Profiler) Run() error {
	return http.ListenAndServe(p.addr, p.mux)
}
