package server

import (
	"net/http"

	"go.uber.org/zap"
)

type Server struct {
	Address string
}

func NewServer(address string) *Server {
	zap.S().Infof("starting httproxy server at %s", address)
	return &Server{Address: address}
}

// AddWildRouter specifically for handling *.m3u8 files
func (s *Server) AddWildRouter() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("heello"))
	})
}

// AddChildProxyRouter for handling routed sub-hls and
// chunks segments
func (s *Server) AddChildProxyRouter() {
	http.HandleFunc("/hlsproxy/:id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("not implimented yet"))
	})
}

func (s *Server) StartAndListen() {
	zap.S().Infof("started listening on %s", s.Address)
	http.ListenAndServe(s.Address, nil)
}
