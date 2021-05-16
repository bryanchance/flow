package render

import "testing"

func TestCalculateRenderSlices(t *testing.T) {
	results, err := calculateRenderSlices(4)
	if err != nil {
		t.Fatal(err)
	}
	// slice 0
	if v := results[0].MinX; v != 0.0 {
		t.Errorf("expected 0.0; received %1f", v)
	}
	if v := results[0].MaxX; v != 0.5 {
		t.Errorf("expected 0.5; received %1f", v)
	}
	if v := results[0].MinY; v != 0.0 {
		t.Errorf("expected 0.0; received %1f", v)
	}
	if v := results[0].MaxY; v != 0.5 {
		t.Errorf("expected 0.5; received %1f", v)
	}
	// slice 1
	if v := results[1].MinX; v != 0.5 {
		t.Errorf("expected 0.5; received %1f", v)
	}
	if v := results[1].MaxX; v != 1.0 {
		t.Errorf("expected 1.0; received %1f", v)
	}
	if v := results[1].MinY; v != 0.0 {
		t.Errorf("expected 0.0; received %1f", v)
	}
	if v := results[1].MaxY; v != 0.5 {
		t.Errorf("expected 0.5; received %1f", v)
	}
	// slice 2
	if v := results[2].MinX; v != 0.0 {
		t.Errorf("expected 0.0; received %1f", v)
	}
	if v := results[2].MaxX; v != 0.5 {
		t.Errorf("expected 0.5; received %1f", v)
	}
	if v := results[2].MinY; v != 0.5 {
		t.Errorf("expected 0.5; received %1f", v)
	}
	if v := results[2].MaxY; v != 1.0 {
		t.Errorf("expected 1.0; received %1f", v)
	}
	// slice 3
	if v := results[3].MinX; v != 0.5 {
		t.Errorf("expected 0.5; received %1f", v)
	}
	if v := results[3].MaxX; v != 1.0 {
		t.Errorf("expected 1.0; received %1f", v)
	}
	if v := results[3].MinY; v != 0.5 {
		t.Errorf("expected 0.5; received %1f", v)
	}
	if v := results[3].MaxY; v != 1.0 {
		t.Errorf("expected 1.0; received %1f", v)
	}
}
