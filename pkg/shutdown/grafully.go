package shutdown

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/meanii/hlsproxy/config"
	"github.com/meanii/hlsproxy/internal/externalcmd"
	"go.uber.org/zap"
)

func EnableGrafullyShutdown() {
	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt)
	signal.Notify(s, syscall.SIGTERM)
	go func() {
		<-s
		// killing all running processes, in order to avoid ghost processes
		for _, cmd := range externalcmd.GloblaActiveCmds {
			zap.S().Infof("closing httpproxy gracefully...\ncmdstring: %s", cmd.GetCmdString())
			syscall.Kill(-cmd.GetProcess().Pid, syscall.SIGINT)
			err := cmd.Process.Kill()
			if err != nil {
				zap.S().Infof("failed to kill :%s", cmd.GetProcess().Pid)
			}
			zap.S().Infof("closed processid: %s", cmd.GetProcess().Pid)
		}
		// Removing output dir
		os.RemoveAll(config.GetConfig("").Config.Output.Dirname)
		os.Exit(0)
	}()
}
