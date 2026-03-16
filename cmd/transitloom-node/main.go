package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/node"
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

	bootstrap, err := node.InspectBootstrap(cfg, time.Now().UTC())
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("transitloom-node config validated for %q using %s", cfg.Identity.Name, *configPath)
	for _, line := range bootstrap.ReportLines() {
		log.Print(line)
	}
	log.Printf("transitloom-node placeholder runtime started; node identity and admission bootstrap are scaffolded, but enrollment, token refresh, and control sessions are not implemented yet")
}
