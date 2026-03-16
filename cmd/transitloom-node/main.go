package main

import (
	"context"
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := node.AttemptBootstrapSession(ctx, cfg, bootstrap)
	for _, line := range session.ReportLines() {
		log.Print(line)
	}
	if err != nil {
		log.Fatal(err)
	}
	if !session.Response.Accepted() {
		log.Printf("transitloom-node bootstrap control session was rejected by coordinator %q", session.Response.CoordinatorName)
		os.Exit(1)
	}

	registrationCtx, registrationCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer registrationCancel()

	registration, err := node.AttemptServiceRegistration(registrationCtx, cfg, bootstrap, session)
	for _, line := range registration.ReportLines() {
		log.Print(line)
	}
	if err != nil {
		log.Fatal(err)
	}
	if !registration.Response.AllRegistered() {
		log.Printf("transitloom-node bootstrap service registration did not fully succeed with coordinator %q", registration.Response.CoordinatorName)
		os.Exit(1)
	}

	log.Printf("transitloom-node bootstrap control and service registration reached coordinator %q; authenticated control sessions, discovery, and associations are still not implemented", registration.Response.CoordinatorName)
}
