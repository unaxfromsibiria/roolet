package main

import (
    "log"
    "roolet/options"
    "roolet/rlserver"
    "os"
)


func main() {
    confPath := os.Getenv("CONF")
    log.Printf("Starting with '%s'...\n", confPath)
    server := rlserver.NewServerCreate(options.JsonOptionSrc{FilePath: confPath})
    server.Run()
}
