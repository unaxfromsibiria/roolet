package rllogger

import (
    "log"
    "fmt"
    "runtime"
    "path/filepath"
    "os"
)

const (
    LogDebug = 0
    LogInfo = 1
    LogWarn = 2
    LogError = 3
    LogTerminate = 4
)

func UseLogDebug() bool {
    return (os.Getenv("RLLOG") == "DEBUG")    
}

func getPath() string {
    result := ""
    if pc, file, line, ok := runtime.Caller(3); ok {
        name := filepath.Base(runtime.FuncForPC(pc).Name())
        result = fmt.Sprintf("%s %s:%d", name, file, line)
    }
    return result
}

func Output(level int, msg string) {
    switch level {
        case LogDebug: {
            if UseLogDebug() {
                log.Printf("DEBUG-> %s - %s\n", getPath(), msg)                
            }
        }
        case LogInfo: {
            log.Printf("INFO-> %s\n", msg)
        }
        case LogWarn: {
            log.Printf("WARN-> %s - %s\n", getPath(), msg)
        }
        case LogError: {
            log.Printf("ERROR-> %s - %s\n", getPath(), msg)
        }
        case LogTerminate: {
            log.Printf("FATAL-> %s - %s\n", getPath(), msg)
            panic("terminate with fatal error!")
        }
        default: {
            log.Println(msg)
        }
    }
}

func Outputf(level int, format string, a...interface{}) {
    msg := fmt.Sprintf(format, a...)
    Output(level, msg)
}

func OutputLines(level int, label string, lines *[]string) {
    msg := fmt.Sprintf("%s:\n", label)
    for _, line := range *lines {
        msg = fmt.Sprintf("%s - %s\n", msg, line)
    }
    Output(level, msg)
}
