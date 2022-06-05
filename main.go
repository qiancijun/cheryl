package main

import (
	"flag"
	"log"

	"github.com/qiancijun/cheryl/cheryl"
	"github.com/qiancijun/cheryl/config"
)

// func filter1(w http.ResponseWriter, r *http.Request) error {
// 	logger.Debug("filter1")
// 	return nil
// }

// func filter2(w http.ResponseWriter, r *http.Request) error {
// 	logger.Debug("filter2")
// 	return nil
// }

// func filter3(w http.ResponseWriter, r *http.Request) error {
// 	logger.Debug("filter3")
// 	return nil
// }

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

	// f1 := filter.NewFilter(filter1)
	// f2 := filter.NewFilter(filter2)
	// f3 := filter.NewFilter(filter3)

	// filter.CreateFilterChain(f1, f2, f3)

	cheryl.Start(config)
	// config.StartServer()
}
