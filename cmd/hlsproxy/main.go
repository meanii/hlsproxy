package main

import (
	"flag"

	"github.com/meanii/hlsproxy/config"
	"github.com/meanii/hlsproxy/internal/server"
	"github.com/meanii/hlsproxy/pkg/logger"
)

func main() {
	addr := flag.String("address", "0.0.0.0:8001", "address of server you want to run on")
	configfile := flag.String("config", "config.yaml", "config file name")

	zaplogger := logger.SetupGlobalLogger()
	defer zaplogger.Sync()

	config.New(*configfile)

	httpServer := server.NewServer(*addr)
	httpServer.AddWildRouter()
	httpServer.StartAndListen()
}
