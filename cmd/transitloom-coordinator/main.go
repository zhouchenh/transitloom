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

	bootstrap, err := pki.InspectCoordinatorBootstrap(cfg)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("transitloom-coordinator config validated for %q using %s", cfg.Identity.Name, *configPath)
	for _, line := range bootstrap.ReportLines() {
		log.Print(line)
	}
	log.Printf("transitloom-coordinator placeholder runtime started; trust bootstrap is scaffolded, but control sessions and issuance workflows are not implemented yet")
}
