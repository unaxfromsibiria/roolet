package main

import (
    "roolet/rllogger"
    "roolet/options"
    "roolet/corelauncher"
    "os"
)


func main() {
    confPath := os.Getenv("CONF")
    optionSrc := options.JsonOptionSrc{FilePath: confPath}
    if option, err := optionSrc.Load(true); err == nil {
	    rllogger.Outputf(rllogger.LogInfo, "Starting with '%s'...\n", confPath)
	    corelauncher.Launch(option)
    } else {
    	rllogger.Outputf(rllogger.LogTerminate, "Option load failed: %s", err)
    }

}
