package main

import (
	"flag"
	"log"

	"com.cheryl/cheryl/config"
	"com.cheryl/cheryl/consistence"
)

func main() {

	configPath := flag.String("config", "config.yaml", "config path")
	flag.Parse()

	config, err := config.ReadConfig(*configPath)
	if err != nil {
		log.Fatalf("read config error: %s", err)
	}

	err = config.Validation()
	if err != nil {
		log.Fatalf("verify config error: %s", err)
	}

	consistence.Start(config)
	// config.StartServer()
}