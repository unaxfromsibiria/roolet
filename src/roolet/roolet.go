package main

import (
	"flag"
	"fmt"
	"os"
	"roolet/corelauncher"
	"roolet/cryptosupport"
	"roolet/options"
	"roolet/rllogger"
)

var toolType, toolData string

type toolMethod func(string, *options.SysOption)

func init() {
	flag.StringVar(&toolType, "tool", "", "Tools command")
	flag.StringVar(&toolData, "data", "", "Data for tools command")
}

func main() {
	confPath := os.Getenv("CONF")
	optionSrc := options.JsonOptionSrc{FilePath: confPath}
	if option, err := optionSrc.Load(true); err == nil {
		flag.Parse()
		rllogger.Outputf(rllogger.LogInfo, "Starting with '%s'...\n", confPath)
		if len(toolType) > 0 {
			// now run for tooling
			var toolMethods map[string]toolMethod = make(map[string]toolMethod)
			toolMethods["jwtcreate"] = cryptosupport.JwtCreate
			toolMethods["jwtcheck"] = cryptosupport.JwtCheck
			toolMethods["keyscheck"] = cryptosupport.KeysSimpleCheck

			if method, exists := toolMethods[toolType]; exists {
				method(toolData, option)
			} else {
				panic(fmt.Sprintf("Unknown tool name requested: '%s'", toolType))
			}
		} else {
			// run as service
			corelauncher.Launch(option)
		}
	} else {
		rllogger.Outputf(rllogger.LogTerminate, "Option load failed: %s", err)
	}
}
