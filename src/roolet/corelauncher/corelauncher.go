package corelauncher

import (
	"roolet/options"
	"roolet/coresupport"
	"roolet/statistic"
	"roolet/rllogger"
	"syscall"
	"os"
	"os/signal"
)

func Launch(option *options.SysOption) {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt)
	signal.Notify(signalChannel, syscall.SIGTERM)
	mustExit := false
	stat := statistic.NewStatistic(*option)
	manager := coresupport.NewCoreWorkerManager(*option, stat)
	manager.Start()
	// wait
	for !mustExit {
		select {
			case newSig := <- signalChannel: {
				if newSig != nil {
					rllogger.Output(rllogger.LogInfo, "Stoping service, now wait..")
					manager.Stop()
				}
			}
			case <- manager.OutSignalChannel: {
				rllogger.Output(rllogger.LogInfo, "Close manager.")
				mustExit = true
			}
		}
    }
	manager.Close()
	stat.Close()
    close(signalChannel)
}
