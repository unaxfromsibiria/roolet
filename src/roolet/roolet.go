package main

import (
	"flag"
    "roolet/rllogger"
    "roolet/options"
    "roolet/corelauncher"
    "roolet/cryptosupport"
    "os"
)

var toolType, toolData string
type toolMethod func(string, options.SysOption)

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
			toolMethods["jwt"] = cryptosupport.JwtCreate
	    	if method, exists := toolMethods[toolType]; exists {
	    		method(toolData, *option)
	    	} else {
	    		panic("Unknown tool name requested.")
	    	}	    	
	    } else {
	    	// run as service
		    corelauncher.Launch(option)	    	
	    }
    } else {
    	rllogger.Outputf(rllogger.LogTerminate, "Option load failed: %s", err)
    }
}
