package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path"
	"syscall"

	"github.com/google/uuid"
	"github.com/meanii/hlsproxy/config"
	"github.com/meanii/hlsproxy/internal/externalcmd"
	"github.com/meanii/hlsproxy/internal/transcoder"
	"go.uber.org/zap"
)

type Server struct {
	Address string
}

type rtmpConfigHTTP struct {
	ID      string `json:"id" validate:"required"`
	RtmpURL string `json:"rtmp_url" validate:"required"`
	Config  struct {
		Varients   []string `json:"varients"`
		VideoCodec string   `json:"video_codec"`
		AudioCodec string   `json:"audio_codec"`
		Audio      bool     `json:"audio" validate:"required"`
	} `json:"config"`
}

func NewServer(address string) *Server {
	zap.S().Infof("starting httproxy server at %s", address)
	return &Server{Address: address}
}

// AddRtmpRouter specifically handling for rtmp as input
// and users can additionally use their own configurations
// POST /rtmp/
// DELETE /rtmp/:id
func (s *Server) AddRtmpRouter() {
	// POST /rtmp resposible to create rtmp pull stream and generate
	// hls stream
	http.HandleFunc("POST /rtmp", func(w http.ResponseWriter, r *http.Request) {
		decode := json.NewDecoder(r.Body)
		var rtmpBody rtmpConfigHTTP
		err := decode.Decode(&rtmpBody)
		if err != nil {
			zap.S().Errorf("failed to decode POST /rtmp body", err)
			w.WriteHeader(400)
			w.Write([]byte("something went wrong!"))
			return
		}

		zap.S().Infof("starting rtpm hlsproxy %+v", rtmpBody)
		tscRunner := transcoder.NewTranscoder(rtmpBody.RtmpURL, rtmpBody.ID)

		zap.S().Infof("user specific config %+v", rtmpBody)
		tscRunner.SetConfig(rtmpBody.Config.Varients, rtmpBody.Config.Audio, rtmpBody.Config.VideoCodec, rtmpBody.Config.AudioCodec)

		_, err = tscRunner.Run()
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("failed to register hlsproxy"))
			return
		}

		w.WriteHeader(200)
		w.Write([]byte("started hlsproxy."))
	})

	// DELETE /rtmp/{id} resposible to termanating running rtmp pulling stream
	http.HandleFunc("DELETE /rtmp/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")

		if id == "" {
			w.WriteHeader(400)
			w.Write([]byte("provide stream id"))
		}
		zap.S().Infof("termanating running hls streaming ID:%s", id)

		activeCmds := externalcmd.GloblaActiveCmds
		zap.S().Infof("total number of runnings hls streaming Count:%s", len(activeCmds))
		for _, cmd := range activeCmds {
			if cmd.StreamID == id {
				cmd.Done()
				zap.S().Infof("closing httpproxy gracefully...\ncmdstring: %s", cmd.GetCmdString())
				syscall.Kill(-cmd.GetProcess().Pid, syscall.SIGINT)
				err := cmd.GetProcess().Kill()
				if err != nil {
					zap.S().Errorf("failed to terminate hls stream ID:%s", cmd.StreamID)
				}
				zap.S().Infof("closed processid: %s", cmd.GetProcess().Pid)
			}
		}

		w.WriteHeader(200)
		w.Write([]byte("success"))
	})
}

// AddHlsRouter specifically for handling hls files
// handling input as HLS only
// GET /*.m3u8
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

// AddFSServerRouter for handling routed sub-hls and
// chunks segments
// GET /hlsproxy/*
func (s *Server) AddFSServerRouter() {
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
