package main

import (
	"flag"
	"log"
	"os"

	"github.com/zhouchenh/transitloom/internal/config"
)

func main() {
	log.SetFlags(0)

	configPath := flag.String("config", "", "Path to the transitloom-coordinator YAML config file")
	flag.Parse()

	if *configPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	cfg, err := config.LoadCoordinator(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	log.Printf("transitloom-coordinator config validated for %q using %s", cfg.Identity.Name, *configPath)
	log.Printf("transitloom-coordinator placeholder runtime started; control-plane and relay behavior are not implemented yet")
}
