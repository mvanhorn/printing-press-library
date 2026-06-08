package acquisition

import (
	"context"
	"fmt"
	"net/http"

	"github.com/mvanhorn/printing-press-library/library/commerce/continente/internal/acquisition/structured"
)

type Capability string

const (
	CapabilitySuggestions Capability = "suggestions"
	CapabilitySearch      Capability = "search"
	CapabilityProduct     Capability = "product"
)

type Adapter string

const (
	AdapterStorefrontHTML  Adapter = "storefront_html"
	AdapterStructuredSCAPI Adapter = "structured_scapi"
	AdapterStructuredOCAPI Adapter = "structured_ocapi"
)

type ProbeState string

const (
	ProbeAvailable    ProbeState = "available"
	ProbeInaccessible ProbeState = "inaccessible"
	ProbeUnavailable  ProbeState = "unavailable"
	ProbeUnknown      ProbeState = "unknown"
)

type CapabilityContract struct {
	Capability       Capability `json:"capability"`
	ActiveAdapter    Adapter    `json:"active_adapter"`
	PreferredAdapter Adapter    `json:"preferred_adapter"`
	StructuredProbe  ProbeState `json:"structured_probe"`
	ProbePath        string     `json:"probe_path,omitempty"`
	Detail           string     `json:"detail,omitempty"`
}

type DiscoveryReport struct {
	Capabilities []CapabilityContract `json:"capabilities"`
}

type Prober interface {
	ProbeGet(ctx context.Context, path string) (int, error)
}

func Discover(ctx context.Context, prober Prober) DiscoveryReport {
	searchProbe := probeStructuredPath(ctx, prober, structured.SCAPISearchProbePath)
	productProbe := probeStructuredPath(ctx, prober, structured.OCAPIProductProbePath)

	return DiscoveryReport{
		Capabilities: []CapabilityContract{
			contractForCapability(CapabilitySuggestions, searchProbe),
			contractForCapability(CapabilitySearch, searchProbe),
			contractForCapability(CapabilityProduct, productProbe),
		},
	}
}

func (r DiscoveryReport) ContractFor(cap Capability) CapabilityContract {
	for _, contract := range r.Capabilities {
		if contract.Capability == cap {
			return contract
		}
	}
	return CapabilityContract{
		Capability:       cap,
		ActiveAdapter:    AdapterStorefrontHTML,
		PreferredAdapter: AdapterStorefrontHTML,
		StructuredProbe:  ProbeUnknown,
		Detail:           "no contract metadata recorded",
	}
}

func (r DiscoveryReport) DoctorSummary() map[string]any {
	capabilities := make([]map[string]any, 0, len(r.Capabilities))
	for _, contract := range r.Capabilities {
		capabilities = append(capabilities, map[string]any{
			"capability":        string(contract.Capability),
			"active_adapter":    string(contract.ActiveAdapter),
			"preferred_adapter": string(contract.PreferredAdapter),
			"structured_probe":  string(contract.StructuredProbe),
			"probe_path":        contract.ProbePath,
			"detail":            contract.Detail,
		})
	}
	return map[string]any{
		"mode":         "storefront_fallback",
		"capabilities": capabilities,
	}
}

type structuredProbe struct {
	path   string
	state  ProbeState
	detail string
}

func probeStructuredPath(ctx context.Context, prober Prober, path string) structuredProbe {
	if prober == nil {
		return structuredProbe{
			path:   path,
			state:  ProbeUnknown,
			detail: "no prober available",
		}
	}
	status, err := prober.ProbeGet(ctx, path)
	if err != nil {
		return structuredProbe{
			path:   path,
			state:  ProbeUnknown,
			detail: fmt.Sprintf("probe failed: %v", err),
		}
	}
	switch status {
	case http.StatusOK:
		return structuredProbe{
			path:   path,
			state:  ProbeAvailable,
			detail: "structured surface responded to probe",
		}
	case http.StatusUnauthorized, http.StatusForbidden:
		return structuredProbe{
			path:   path,
			state:  ProbeInaccessible,
			detail: fmt.Sprintf("structured surface exists but is not accessible from the current public context (HTTP %d)", status),
		}
	default:
		return structuredProbe{
			path:   path,
			state:  ProbeUnavailable,
			detail: fmt.Sprintf("structured surface probe returned HTTP %d", status),
		}
	}
}

func contractForCapability(cap Capability, probe structuredProbe) CapabilityContract {
	preferred := AdapterStorefrontHTML
	detail := "using storefront HTML adapter"
	switch probe.state {
	case ProbeAvailable:
		preferred = AdapterStructuredSCAPI
		if cap == CapabilityProduct {
			preferred = AdapterStructuredOCAPI
		}
		detail = "structured surface detected, but storefront adapter remains active until structured fetch implementation is promoted"
	case ProbeInaccessible:
		detail = "structured surface detected but inaccessible; using storefront HTML adapter"
	case ProbeUnavailable:
		detail = "structured surface not detected; using storefront HTML adapter"
	case ProbeUnknown:
		detail = "structured probe inconclusive; using storefront HTML adapter"
	}
	return CapabilityContract{
		Capability:       cap,
		ActiveAdapter:    AdapterStorefrontHTML,
		PreferredAdapter: preferred,
		StructuredProbe:  probe.state,
		ProbePath:        probe.path,
		Detail:           detail,
	}
}
