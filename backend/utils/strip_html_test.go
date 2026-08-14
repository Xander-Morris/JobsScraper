package utils

import "testing"

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "paragraphs become a blank line apart",
			input: "<p>Company Overview:</p><p>We build things.</p>",
			want:  "Company Overview:\n\nWe build things.",
		},
		{
			name:  "br becomes a single newline",
			input: "Line one<br>Line two<br/>Line three",
			want:  "Line one\nLine two\nLine three",
		},
		{
			name:  "list items become bulleted lines",
			input: "<ul><li>First</li><li>Second</li></ul>",
			want:  "• First\n\n• Second",
		},
		{
			name:  "entities decode instead of round-tripping to escaped form",
			input: "<p>We&#39;re hiring &amp; growing</p>",
			want:  "We're hiring & growing",
		},
		{
			name:  "inline tags collapse without inserting breaks",
			input: "<p>Great <b>role</b></p>",
			want:  "Great role",
		},
		{
			name:  "plain text passes through unchanged",
			input: "Test things",
			want:  "Test things",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripHTML(tt.input); got != tt.want {
				t.Errorf("StripHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
