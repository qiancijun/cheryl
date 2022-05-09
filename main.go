package main

import (
	"log"
	"com.cheryl/cheryl/config"
)

func main() {
	config, err := config.ReadConfig("config.yaml")
	if err != nil {
		log.Fatalf("read config error: %s", err)
	}

	err = config.Validation()
	if err != nil {
		log.Fatalf("verify config error: %s", err)
	}
	config.StartServer()
}