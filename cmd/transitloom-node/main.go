package main

import (
	"flag"
	"log"
	"os"

	"github.com/zhouchenh/transitloom/internal/config"
)

func main() {
	log.SetFlags(0)

	configPath := flag.String("config", "", "Path to the transitloom-node YAML config file")
	flag.Parse()

	if *configPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	cfg, err := config.LoadNode(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	log.Printf("transitloom-node config validated for %q using %s", cfg.Identity.Name, *configPath)
	log.Printf("transitloom-node placeholder runtime started; control sessions and service carriage are not implemented yet")
}
