package server

import (
	"strings"
	"testing"
)

func TestParseTriviaRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     triviaRequest
		wantErr bool
	}{
		{"resume", triviaRequest{Source: "resume", Count: 5, Difficulty: "easy"}, false},
		{"prompt", triviaRequest{Source: "prompt", Prompt: "  React hooks ", Count: 10, Difficulty: "hard"}, false},
		{"unknown source", triviaRequest{Source: "job", Count: 5, Difficulty: "easy"}, true},
		{"bad count", triviaRequest{Source: "resume", Count: 7, Difficulty: "easy"}, true},
		{"bad difficulty", triviaRequest{Source: "resume", Count: 5, Difficulty: "expert"}, true},
		{"missing prompt", triviaRequest{Source: "prompt", Prompt: "   ", Count: 5, Difficulty: "easy"}, true},
		{"oversized prompt", triviaRequest{Source: "prompt", Prompt: strings.Repeat("a", 301), Count: 5, Difficulty: "easy"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseTriviaRequest(&tt.req)

			if (err != nil) != tt.wantErr {
				t.Fatalf("parseTriviaRequest() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseTriviaRequestNormalizesPrompt(t *testing.T) {
	prompt := triviaRequest{Source: "prompt", Prompt: "  React hooks ", Count: 5, Difficulty: "easy"}
	if err := parseTriviaRequest(&prompt); err != nil || prompt.Prompt != "React hooks" {
		t.Errorf("prompt = %q err = %v, want trimmed", prompt.Prompt, err)
	}

	resume := triviaRequest{Source: "resume", Prompt: "ignored", Count: 5, Difficulty: "easy"}
	if err := parseTriviaRequest(&resume); err != nil || resume.Prompt != "" {
		t.Errorf("resume prompt = %q err = %v, want cleared", resume.Prompt, err)
	}
}
