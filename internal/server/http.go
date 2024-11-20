package server

import (
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/google/uuid"
	"github.com/meanii/hlsproxy/config"
	"github.com/meanii/hlsproxy/internal/transcoder"
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
func (s *Server) AddHlsRouter() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		sourceHlsURL := r.URL
		originServerHost, _ := url.Parse(config.GetConfig("").Config.OriginServer.URL)
		sourceHlsURL.Host = originServerHost.Host
		sourceHlsURL.Scheme = originServerHost.Scheme
		zap.S().Infof("source hls url registering %s", sourceHlsURL.String())

		id := uuid.New().String()
		transcoderRunner := transcoder.NewTranscoder(sourceHlsURL.String(), id)
		m3u8string, err := transcoderRunner.Run()
		if err != nil {
			zap.S().Errorf("failed to start trasncoder, Error: %s", err)
		}

		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Write([]byte(m3u8string))
	})
}

// AddChildProxyRouter for handling routed sub-hls and
// chunks segments
func (s *Server) AddChildProxyRouter() {
	wd, _ := os.Getwd()
	fspath := path.Join(wd, config.GlobalConfigInstance.Config.Output.Dirname)
	zap.S().Infof("registering file server %s", fspath)
	fs := http.FileServer(http.Dir(fspath))
	http.Handle("/hlsproxy/", http.StripPrefix("/hlsproxy", fs))
}

func (s *Server) StartAndListen() {
	zap.S().Infof("started listening on %s", s.Address)
	http.ListenAndServe(s.Address, nil)
}
