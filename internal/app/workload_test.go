package app

import (
	"math/rand"
	"slices"
	"testing"
	"time"
)

func TestBuildOperationsIncludesCartWrites(t *testing.T) {
	cfg := Config{Profile: "write-light", WriteMode: "cart"}
	ops := buildOperations(cfg, "run-1")

	names := operationNames(ops)
	for _, want := range []string{"cart_add_item", "cart_update_item", "cart_cleanup"} {
		if !slices.Contains(names, want) {
			t.Fatalf("operations missing %q: %v", want, names)
		}
	}
}

func TestBuildOperationsReportingExcludesWrite(t *testing.T) {
	cfg := Config{Profile: "reporting", WriteMode: "cart"}
	ops := buildOperations(cfg, "run-1")

	for _, op := range ops {
		if op.Kind == "write" {
			t.Fatalf("reporting profile should exclude write ops, got %q", op.Name)
		}
	}
}

func TestBuildOperationsAppliesProfileMultipliers(t *testing.T) {
	mixed := buildOperations(Config{Profile: "mixed", WriteMode: "off"}, "run-1")
	reporting := buildOperations(Config{Profile: "reporting", WriteMode: "off"}, "run-1")

	mixedWeight := weightFor(mixed, "sales_dashboard")
	reportingWeight := weightFor(reporting, "sales_dashboard")
	if reportingWeight <= mixedWeight {
		t.Fatalf("reporting weight = %d, mixed weight = %d; expected reporting to boost reports", reportingWeight, mixedWeight)
	}
}

func TestChooseOperationSkewsTowardPersonaPreference(t *testing.T) {
	cfg := Config{Profile: "mixed", WriteMode: "cart"}
	ops := buildOperations(cfg, "run-1")
	rng := rand.New(rand.NewSource(42))

	counts := map[string]int{}
	const iterations = 5000
	for range iterations {
		persona := Persona{
			Type:           "shopper",
			WeightModifier: map[string]float64{"catalog_search": 1.8, "cart_add_item": 1.7},
		}
		op := chooseOperation(ops, persona, rng)
		counts[op.Name]++
	}

	if counts["catalog_search"] <= counts["employee_managers"] {
		t.Fatalf("shopper should prefer catalog_search, counts = %#v", counts)
	}
}

func TestThinkDurationRespectsBounds(t *testing.T) {
	cfg := Config{ThinkMin: 100 * time.Millisecond, ThinkMax: 200 * time.Millisecond}
	persona := Persona{Tempo: 1.0}
	rng := rand.New(rand.NewSource(7))

	for range 100 {
		got := thinkDuration(cfg, persona, rng)
		if got < cfg.ThinkMin || got > cfg.ThinkMax {
			t.Fatalf("thinkDuration() = %s, want between %s and %s", got, cfg.ThinkMin, cfg.ThinkMax)
		}
	}
}

func TestNewPersonaIsDeterministicForSeed(t *testing.T) {
	rng1 := rand.New(rand.NewSource(99))
	rng2 := rand.New(rand.NewSource(99))

	p1 := newPersona(5, rng1)
	p2 := newPersona(5, rng2)

	if p1.Type != p2.Type || p1.Tempo != p2.Tempo {
		t.Fatalf("expected deterministic persona, got %#v vs %#v", p1, p2)
	}
	if p1.ID != 5 {
		t.Fatalf("persona ID = %d, want 5", p1.ID)
	}
}

func operationNames(ops []Operation) []string {
	names := make([]string, len(ops))
	for i, op := range ops {
		names[i] = op.Name
	}
	return names
}

func weightFor(ops []Operation, name string) int {
	for _, op := range ops {
		if op.Name == name {
			return op.Weight
		}
	}
	return 0
}
