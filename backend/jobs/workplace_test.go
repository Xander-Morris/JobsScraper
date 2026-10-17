package jobs

import (
	"encoding/json"
	"testing"
)

func TestParseWorkplaceType(t *testing.T) {
	tests := []struct {
		in     string
		want   WorkplaceType
		wantOK bool
	}{
		{"", Unknown, true},
		{"  ", Unknown, true},
		{"remote", Remote, true},
		{" Remote ", Remote, true},
		{"HYBRID", Hybrid, true},
		{"in_person", InPerson, true},
		{"in-person", InPerson, true},
		{"inperson", InPerson, true},
		{"onsite", InPerson, true},
		{"on-site", InPerson, true},
		{"on_site", InPerson, true},
		{"mars", Unknown, false},
	}

	for _, tt := range tests {
		got, ok := ParseWorkplaceType(tt.in)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("ParseWorkplaceType(%q) = (%v, %v), want (%v, %v)", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestWorkplaceFromLocation(t *testing.T) {
	tests := map[string]WorkplaceType{
		"Remote - US":   Remote,
		"REMOTE":        Remote,
		"Fully remote!": Remote,
		"New York, NY":  Unknown,
		"":              Unknown,
	}

	for in, want := range tests {
		if got := WorkplaceFromLocation(in); got != want {
			t.Errorf("WorkplaceFromLocation(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestWorkplaceTypeJSON(t *testing.T) {
	for _, w := range []WorkplaceType{Unknown, Remote, Hybrid, InPerson} {
		raw, err := json.Marshal(w)
		if err != nil {
			t.Fatal(err)
		}

		if want := `"` + w.String() + `"`; string(raw) != want {
			t.Errorf("Marshal(%v) = %s, want %s", w, raw, want)
		}

		var back WorkplaceType
		if err := json.Unmarshal(raw, &back); err != nil || back != w {
			t.Errorf("round trip %v -> %v (%v)", w, back, err)
		}
	}

	if got := WorkplaceType(42).String(); got != "unknown" {
		t.Errorf("WorkplaceType(42).String() = %q", got)
	}

	w := Remote
	if err := json.Unmarshal([]byte(`"spaceship"`), &w); err != nil || w != Unknown {
		t.Errorf("unknown string = %v, %v", w, err)
	}

	if err := json.Unmarshal([]byte(`1`), &w); err == nil {
		t.Error("number should error")
	}
}

func TestJobJSONOmitsEmptyFields(t *testing.T) {
	raw, err := json.Marshal(Job{Title: "T"})
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	_ = json.Unmarshal(raw, &m)

	for _, key := range []string{"posted_at", "description", "match_score", "match_percent"} {
		if _, ok := m[key]; ok {
			t.Errorf("%s should be omitted when empty: %s", key, raw)
		}
	}

	if m["workplace_type"] != "unknown" || m["applied"] != false {
		t.Errorf("json = %s", raw)
	}
}
