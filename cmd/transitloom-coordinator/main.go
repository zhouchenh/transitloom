package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/coordinator"
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
	listener, err := coordinator.NewBootstrapListener(cfg, bootstrap)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("transitloom-coordinator config validated for %q using %s", cfg.Identity.Name, *configPath)
	for _, line := range bootstrap.ReportLines() {
		log.Print(line)
	}
	for _, line := range listener.ReportLines() {
		log.Print(line)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("transitloom-coordinator minimal bootstrap control runtime started; stop with SIGINT or SIGTERM")
	if err := listener.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
