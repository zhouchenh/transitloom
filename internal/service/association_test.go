package service_test

import (
	"testing"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/service"
)

func TestAssociationIntentValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		intent  service.AssociationIntent
		wantErr bool
	}{
		{
			name: "valid intent",
			intent: service.AssociationIntent{
				SourceService:      service.Identity{Name: "wg-home", Type: config.ServiceTypeRawUDP},
				DestinationNode:    "node-b",
				DestinationService: service.Identity{Name: "wg-home", Type: config.ServiceTypeRawUDP},
			},
			wantErr: false,
		},
		{
			name: "missing source service name",
			intent: service.AssociationIntent{
				SourceService:      service.Identity{Name: "", Type: config.ServiceTypeRawUDP},
				DestinationNode:    "node-b",
				DestinationService: service.Identity{Name: "wg-home", Type: config.ServiceTypeRawUDP},
			},
			wantErr: true,
		},
		{
			name: "missing destination node",
			intent: service.AssociationIntent{
				SourceService:      service.Identity{Name: "wg-home", Type: config.ServiceTypeRawUDP},
				DestinationNode:    "",
				DestinationService: service.Identity{Name: "wg-home", Type: config.ServiceTypeRawUDP},
			},
			wantErr: true,
		},
		{
			name: "missing destination service name",
			intent: service.AssociationIntent{
				SourceService:      service.Identity{Name: "wg-home", Type: config.ServiceTypeRawUDP},
				DestinationNode:    "node-b",
				DestinationService: service.Identity{Name: "", Type: config.ServiceTypeRawUDP},
			},
			wantErr: true,
		},
		{
			name: "invalid source service type",
			intent: service.AssociationIntent{
				SourceService:      service.Identity{Name: "wg-home", Type: "tcp"},
				DestinationNode:    "node-b",
				DestinationService: service.Identity{Name: "wg-home", Type: config.ServiceTypeRawUDP},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.intent.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildAssociationIntents(t *testing.T) {
	t.Parallel()

	cfg := config.NodeConfig{
		Services: []config.ServiceConfig{
			{Name: "wg-home", Type: config.ServiceTypeRawUDP},
			{Name: "wg-office", Type: config.ServiceTypeRawUDP},
		},
		Associations: []config.AssociationConfig{
			{
				SourceService:      "wg-home",
				DestinationNode:    "node-b",
				DestinationService: "wg-home",
			},
		},
	}

	intents, err := service.BuildAssociationIntents(cfg)
	if err != nil {
		t.Fatalf("BuildAssociationIntents() error = %v", err)
	}
	if len(intents) != 1 {
		t.Fatalf("len(intents) = %d, want 1", len(intents))
	}

	intent := intents[0]
	if intent.SourceService.Name != "wg-home" {
		t.Fatalf("SourceService.Name = %q, want %q", intent.SourceService.Name, "wg-home")
	}
	if intent.SourceService.Type != config.ServiceTypeRawUDP {
		t.Fatalf("SourceService.Type = %q, want %q", intent.SourceService.Type, config.ServiceTypeRawUDP)
	}
	if intent.DestinationNode != "node-b" {
		t.Fatalf("DestinationNode = %q, want %q", intent.DestinationNode, "node-b")
	}
	if intent.DestinationService.Name != "wg-home" {
		t.Fatalf("DestinationService.Name = %q, want %q", intent.DestinationService.Name, "wg-home")
	}
	// Destination type defaults to raw-udp for v1.
	if intent.DestinationService.Type != config.ServiceTypeRawUDP {
		t.Fatalf("DestinationService.Type = %q, want %q", intent.DestinationService.Type, config.ServiceTypeRawUDP)
	}
}

func TestBuildAssociationIntentsRejectsUnknownSourceService(t *testing.T) {
	t.Parallel()

	cfg := config.NodeConfig{
		Services: []config.ServiceConfig{
			{Name: "wg-home", Type: config.ServiceTypeRawUDP},
		},
		Associations: []config.AssociationConfig{
			{
				SourceService:      "nonexistent",
				DestinationNode:    "node-b",
				DestinationService: "wg-home",
			},
		},
	}

	_, err := service.BuildAssociationIntents(cfg)
	if err == nil {
		t.Fatal("BuildAssociationIntents() should reject unknown source service")
	}
}

func TestBuildAssociationIntentsReturnsNilWhenEmpty(t *testing.T) {
	t.Parallel()

	cfg := config.NodeConfig{
		Services: []config.ServiceConfig{
			{Name: "wg-home", Type: config.ServiceTypeRawUDP},
		},
	}

	intents, err := service.BuildAssociationIntents(cfg)
	if err != nil {
		t.Fatalf("BuildAssociationIntents() error = %v", err)
	}
	if intents != nil {
		t.Fatalf("intents = %v, want nil", intents)
	}
}

func TestAssociationKey(t *testing.T) {
	t.Parallel()

	key := service.AssociationKey(
		"node-a",
		service.Identity{Name: "wg-home", Type: config.ServiceTypeRawUDP},
		"node-b",
		service.Identity{Name: "wg-home", Type: config.ServiceTypeRawUDP},
	)

	want := "node-a/wg-home:raw-udp->node-b/wg-home:raw-udp"
	if key != want {
		t.Fatalf("AssociationKey() = %q, want %q", key, want)
	}
}
