package corelauncher

import (
	"os"
	"os/signal"
	"roolet/connectionserver"
	"roolet/coresupport"
	"roolet/coremethods"
	"roolet/options"
	"roolet/rllogger"
	"roolet/statistic"
	"syscall"
)

func Launch(option *options.SysOption) {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt)
	signal.Notify(signalChannel, syscall.SIGTERM)
	coremethods.Setup()
	mustExit := false
	stat := statistic.NewStatistic(*option)
	manager := coresupport.NewCoreWorkerManager(*option, stat)
	server := connectionserver.NewServer(*option, stat)
	manager.Start(server)
	server.Start(manager)
	// wait
	for !mustExit {
		select {
		case newSig := <-signalChannel:
			{
				if newSig != nil {
					rllogger.Output(rllogger.LogInfo, "Stoping service, now wait..")
					server.Stop()
					manager.Stop()
				}
			}
		case <-manager.OutSignalChannel:
			{
				rllogger.Output(rllogger.LogInfo, "Close manager.")
				mustExit = true
			}
		}
	}
	manager.Close()
	stat.Close()
	close(signalChannel)
}
