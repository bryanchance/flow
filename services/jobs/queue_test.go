package render

import "testing"

func TestCalculateRenderSlices4(t *testing.T) {
	expectedSlices := 4
	results, err := calculateRenderSlices(expectedSlices)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != expectedSlices {
		t.Fatalf("expected %d slices; received %d", expectedSlices, len(results))
	}
}

func TestCalculateRenderSlices5(t *testing.T) {
	expectedSlices := 6
	results, err := calculateRenderSlices(5)
	if err != nil {
		t.Fatal(err)
	}
	if v := len(results); v != expectedSlices {
		t.Fatalf("expected %d results; received %d", expectedSlices, v)
	}
}

func TestCalculateRenderSlices8(t *testing.T) {
	expectedSlices := 8
	results, err := calculateRenderSlices(expectedSlices)
	if err != nil {
		t.Fatal(err)
	}
	if v := len(results); v != expectedSlices {
		t.Fatalf("expected %d results; received %d", expectedSlices, v)
	}
}

func TestCalculateRenderSlices10(t *testing.T) {
	expectedSlices := 10
	results, err := calculateRenderSlices(expectedSlices)
	if err != nil {
		t.Fatal(err)
	}
	if v := len(results); v != expectedSlices {
		t.Fatalf("expected %d results; received %d", expectedSlices, v)
	}
}
