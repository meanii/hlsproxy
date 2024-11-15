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

func (s *Server) AddWildRouter() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("heello"))
	})
}

func (s *Server) StartAndListen() {
	zap.S().Infof("started listening on %s", s.Address)
	http.ListenAndServe(s.Address, nil)
}
