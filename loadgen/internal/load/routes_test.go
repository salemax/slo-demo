package load

import (
	"reflect"
	"testing"
)

func TestParseRoutes(t *testing.T) {
	got, err := ParseRoutes(" /api/fast=80, /api/slow = 20 ,/x=0")
	if err != nil {
		t.Fatal(err)
	}
	want := []Route{{"/api/fast", 80}, {"/api/slow", 20}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseRoutesRejects(t *testing.T) {
	for _, in := range []string{
		"",
		"/api/fast",
		"/api/fast=-1",
		"/api/fast=x",
		"api/fast=1",
		"/api/fast=1,/api/fast=2",
		"/api/fast=0",
		"/healthz=1",
		"/metrics=1",
		"/admin=1",
		"/admin/faults=1",
	} {
		if _, err := ParseRoutes(in); err == nil {
			t.Errorf("ParseRoutes(%q): want error, got nil", in)
		}
	}
}

// Walking every n in [0, total) gives the exact split, with no randomness.
func TestPickerSplitExact(t *testing.T) {
	p := NewPicker([]Route{{"/a", 80}, {"/b", 20}, {"/c", 1}})
	counts := map[string]int{}
	for n := range p.total {
		counts[p.pick(n)]++
	}
	want := map[string]int{"/a": 80, "/b": 20, "/c": 1}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("got %v, want %v", counts, want)
	}
}

// With the real random source the split is close to the weights.
func TestPickerSplitRandom(t *testing.T) {
	p := NewPicker([]Route{{"/api/fast", 80}, {"/api/slow", 20}})
	const n = 100_000
	slow := 0
	for range n {
		if p.Pick() == "/api/slow" {
			slow++
		}
	}
	// Expected 20000, sd = sqrt(n*0.2*0.8) ~ 126; 1000 is ~8 sd.
	if slow < 19_000 || slow > 21_000 {
		t.Fatalf("/api/slow picked %d of %d, want ~20000", slow, n)
	}
}
