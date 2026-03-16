package main

import (
	"flag"
	"log"
	"os"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/pki"
)

func main() {
	log.SetFlags(0)

	configPath := flag.String("config", "", "Path to the transitloom-root YAML config file")
	flag.Parse()

	if *configPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	cfg, err := config.LoadRoot(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	bootstrap, err := pki.InspectRootBootstrap(cfg)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("transitloom-root config validated for %q using %s", cfg.Identity.Name, *configPath)
	for _, line := range bootstrap.ReportLines() {
		log.Print(line)
	}
	log.Printf("transitloom-root placeholder runtime started; trust bootstrap is scaffolded, but issuance workflows are not implemented yet")
}
