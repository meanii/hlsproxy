package main

import (
	"flag"

	"github.com/meanii/hlsproxy/config"
	"github.com/meanii/hlsproxy/internal/server"
	"github.com/meanii/hlsproxy/pkg/logger"
	"github.com/meanii/hlsproxy/pkg/shutdown"
)

func main() {
	addr := flag.String("address", "0.0.0.0:8001", "address of server you want to run on")
	configfile := flag.String("config", "config.yaml", "config file name")

	zaplogger := logger.SetupGlobalLogger()
	defer zaplogger.Sync()

	_ = config.GetConfig(*configfile)

	shutdown.EnableGrafullyShutdown()

	httpServer := server.NewServer(*addr)

	// adding routers
	httpServer.AddRtmpRouter()
	httpServer.AddHlsRouter()
	httpServer.AddFSServerRouter()
	httpServer.StartAndListen()
}
