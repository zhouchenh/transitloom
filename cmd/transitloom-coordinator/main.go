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
	"github.com/zhouchenh/transitloom/internal/status"
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

	// Start the read-only status server if configured. The status server
	// exposes current coordinator runtime state (bootstrap trust readiness,
	// registered services, association records) at the observability.status
	// listen address. This is what 'tlctl coordinator status' queries.
	//
	// The status server is intentionally read-only. It exposes existing
	// runtime state without redefining it. The output preserves the
	// architectural distinction between service registration state and
	// association state — they are separate sections in the output.
	if cfg.Observability.Status.Enabled && cfg.Observability.Status.Listen != "" {
		capturedBootstrap := bootstrap
		capturedListener := listener
		statusServer := status.NewStatusServer(func() []string {
			// Snapshot aggregates distinct state categories without merging them:
			// coordinator trust bootstrap readiness (from disk inspection) and
			// current service registry + association store (in-memory runtime state).
			var lines []string
			lines = append(lines, capturedBootstrap.ReportLines()...)
			lines = append(lines, capturedListener.RuntimeSummaryLines()...)
			return lines
		})
		go func() {
			if err := statusServer.ListenAndServe(ctx, cfg.Observability.Status.Listen); err != nil {
				log.Printf("transitloom-coordinator status server error: %v", err)
			}
		}()
		log.Printf("transitloom-coordinator status endpoint active at http://%s/status", cfg.Observability.Status.Listen)
	}

	log.Printf("transitloom-coordinator minimal bootstrap control runtime started; stop with SIGINT or SIGTERM")
	if err := listener.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
