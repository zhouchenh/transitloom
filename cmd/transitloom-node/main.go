package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
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

	// Track accepted association results for direct-path activation.
	var assocResults []node.AssociationResultEntry

	if len(cfg.Associations) > 0 {
		associationCtx, associationCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer associationCancel()

		association, assocErr := node.AttemptAssociation(associationCtx, cfg, bootstrap, session)
		for _, line := range association.ReportLines() {
			log.Print(line)
		}
		if assocErr != nil {
			log.Fatal(assocErr)
		}
		if !association.Response.AllCreated() {
			log.Printf("transitloom-node bootstrap association did not fully succeed with coordinator %q", association.Response.CoordinatorName)
			os.Exit(1)
		}

		// Extract accepted association results for direct-path activation.
		assocResults = extractAssociationResults(association.Response.Results)
	}

	// Activate direct-path carriage for associations with direct_endpoint.
	// This is the runtime integration that makes WireGuard-over-mesh work
	// on a direct path. WireGuard remains standard — Transitloom provides
	// the local ingress endpoints and carries raw UDP packets with zero
	// in-band overhead.
	runtime := node.NewDirectPathRuntime()
	defer runtime.Carrier.StopAll()

	inputs := node.BuildAssociationActivationInputs(cfg, assocResults)
	if len(inputs) > 0 {
		runtimeCtx, runtimeCancel := context.WithCancel(context.Background())
		defer runtimeCancel()

		directResult := node.ActivateDirectPaths(runtimeCtx, cfg, runtime, inputs)
		for _, line := range directResult.ReportLines() {
			log.Print(line)
		}

		if directResult.TotalActive > 0 {
			log.Printf("transitloom-node direct-path carriage active: %d association(s); WireGuard can use Transitloom local ingress ports as peer endpoints", directResult.TotalActive)

			// Stay running until signaled so carriage continues.
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
			log.Printf("transitloom-node shutting down")
			runtimeCancel()
			return
		}
	}

	log.Printf("transitloom-node bootstrap control, service registration, and association reached coordinator %q; direct-path carriage requires associations with direct_endpoint configured", registration.Response.CoordinatorName)
}

// extractAssociationResults converts controlplane association results to the
// node-level AssociationResultEntry format needed for direct-path activation.
func extractAssociationResults(results []controlplane.AssociationResult) []node.AssociationResultEntry {
	entries := make([]node.AssociationResultEntry, 0, len(results))
	for _, r := range results {
		entries = append(entries, node.AssociationResultEntry{
			AssociationID:      r.AssociationID,
			SourceServiceName:  r.SourceServiceName,
			DestinationNode:    r.DestinationNode,
			DestinationService: r.DestinationService,
			Accepted:           r.Outcome == controlplane.AssociationResultOutcomeCreated,
		})
	}
	return entries
}
