package main

import (
	"log"

	"mpbt3025/src/config"
	"mpbt3025/src/server"
)

func main() {
	cfg := config.ParseFlags()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}
	server.Run(cfg)
}
