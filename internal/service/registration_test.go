package service_test

import (
	"strings"
	"testing"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/service"
)

func TestBuildRegistrationsResolvesRequestedLocalIngressSeparatelyFromBinding(t *testing.T) {
	t.Parallel()

	registrations, err := service.BuildRegistrations(config.NodeConfig{
		LocalIngress: config.LocalIngressPolicyConfig{
			DefaultMode:     config.IngressModeDeterministicRange,
			RangeStart:      61000,
			RangeEnd:        61999,
			LoopbackAddress: "127.0.0.1",
		},
		Services: []config.ServiceConfig{
			{
				Name:         "wg-home",
				Type:         config.ServiceTypeRawUDP,
				Discoverable: true,
				Binding: config.ServiceBindingConfig{
					Address: "127.0.0.1",
					Port:    51820,
				},
				Ingress: &config.ServiceIngressConfig{
					Mode: config.IngressModeDeterministicRange,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildRegistrations() error = %v", err)
	}

	if got := len(registrations); got != 1 {
		t.Fatalf("len(registrations) = %d, want 1", got)
	}

	registration := registrations[0]
	if registration.Identity.Name != "wg-home" {
		t.Fatalf("registration.Identity.Name = %q, want %q", registration.Identity.Name, "wg-home")
	}
	if registration.Binding.LocalTarget.Port != 51820 {
		t.Fatalf("registration.Binding.LocalTarget.Port = %d, want 51820", registration.Binding.LocalTarget.Port)
	}
	if registration.RequestedLocalIngress == nil {
		t.Fatal("registration.RequestedLocalIngress = nil, want resolved ingress intent")
	}
	if registration.RequestedLocalIngress.RangeStart != 61000 || registration.RequestedLocalIngress.RangeEnd != 61999 {
		t.Fatalf("registration.RequestedLocalIngress range = %d-%d, want 61000-61999", registration.RequestedLocalIngress.RangeStart, registration.RequestedLocalIngress.RangeEnd)
	}
	if registration.RequestedLocalIngress.LoopbackAddress != "127.0.0.1" {
		t.Fatalf("registration.RequestedLocalIngress.LoopbackAddress = %q, want %q", registration.RequestedLocalIngress.LoopbackAddress, "127.0.0.1")
	}
	if registration.RequestedLocalIngress.StaticPort != 0 {
		t.Fatalf("registration.RequestedLocalIngress.StaticPort = %d, want 0", registration.RequestedLocalIngress.StaticPort)
	}
}

func TestRegistrationValidateRejectsInvalidDeclarations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		registration service.Registration
		wantErr      string
	}{
		{
			name: "invalid local target address",
			registration: service.Registration{
				Identity: service.Identity{
					Name: "dns-cache",
					Type: config.ServiceTypeRawUDP,
				},
				Binding: service.Binding{
					LocalTarget: service.LocalTarget{
						Address: "not-an-ip",
						Port:    5300,
					},
				},
			},
			wantErr: "local_target.address",
		},
		{
			name: "non loopback requested local ingress",
			registration: service.Registration{
				Identity: service.Identity{
					Name: "wg-home",
					Type: config.ServiceTypeRawUDP,
				},
				Binding: service.Binding{
					LocalTarget: service.LocalTarget{
						Address: "127.0.0.1",
						Port:    51820,
					},
				},
				RequestedLocalIngress: &service.LocalIngressIntent{
					Mode:            config.IngressModeDeterministicRange,
					LoopbackAddress: "10.0.0.10",
					RangeStart:      61000,
					RangeEnd:        61999,
				},
			},
			wantErr: "loopback_address",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.registration.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
