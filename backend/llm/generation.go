package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ResumeProfile struct {
	FullName       string
	Summary        string
	Skills         []string
	WorkExperience []WorkExperienceEntry
	Projects       []ProjectEntry
}

type JobPosting struct {
	Title       string
	Company     string
	Description string
}

type GeneratedContent struct {
	CoverLetter     string   `json:"cover_letter"`
	TailoredBullets []string `json:"tailored_bullets"`
}

const generationPrompt = "Using the candidate's resume information and the job posting below, write a tailored " +
	"cover letter and a short list of resume bullet points reworded to highlight the experience most relevant to " +
	"this job. Base every claim only on the resume information given — do not invent skills, employers, titles, " +
	"or accomplishments that aren't present in it. Keep the cover letter to 3-4 short paragraphs, specific to the " +
	"role and company rather than generic. Tailored bullets should reuse the candidate's real achievements and " +
	"numbers where available, rewritten to emphasize relevance to this job's requirements — do not fabricate new ones."

func GenerateApplicationContent(ctx context.Context, profile ResumeProfile, job JobPosting) (*GeneratedContent, error) {
	reqBody := map[string]any{
		"model": ollamaModel(),
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": generationPrompt + "\n\n" + formatResumeProfile(profile) + "\n\n" + formatJobPosting(job),
			},
		},
		"format": generationSchema(),
		"stream": false,
		"options": map[string]any{
			"num_ctx":     16384,
			"temperature": 0,
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ollamaBaseURL()+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call ollama (is `ollama serve` running at %s?): %w", ollamaBaseURL(), err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ollama response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(respBody))
	}

	return parseGenerationResponse(respBody)
}

// EmbeddingText renders a resume profile as the same compact, information-dense
// plain text GenerateApplicationContent feeds to the LLM — a good representation
// to embed, since it's already a summary of everything that's actually in the
// resume.
func EmbeddingText(profile ResumeProfile) string {
	return formatResumeProfile(profile)
}

func formatResumeProfile(p ResumeProfile) string {
	var sb strings.Builder

	sb.WriteString("Candidate resume:\n")
	if p.FullName != "" {
		fmt.Fprintf(&sb, "Name: %s\n", p.FullName)
	}
	if p.Summary != "" {
		fmt.Fprintf(&sb, "Summary: %s\n", p.Summary)
	}
	if len(p.Skills) > 0 {
		fmt.Fprintf(&sb, "Skills: %s\n", strings.Join(p.Skills, ", "))
	}

	for _, we := range p.WorkExperience {
		fmt.Fprintf(&sb, "\nRole: %s at %s (%s to %s)\n", we.JobTitle, we.Company, we.StartDate, orPresent(we.EndDate))
		for _, bullet := range we.Bullets {
			fmt.Fprintf(&sb, "- %s\n", bullet)
		}
	}

	for _, proj := range p.Projects {
		fmt.Fprintf(&sb, "\nProject: %s", proj.Name)
		if len(proj.Technologies) > 0 {
			fmt.Fprintf(&sb, " (%s)", strings.Join(proj.Technologies, ", "))
		}
		sb.WriteString("\n")
		for _, bullet := range proj.Bullets {
			fmt.Fprintf(&sb, "- %s\n", bullet)
		}
	}

	return sb.String()
}

func orPresent(endDate string) string {
	if endDate == "" {
		return "present"
	}

	return endDate
}

func formatJobPosting(j JobPosting) string {
	return fmt.Sprintf("Job posting:\nTitle: %s\nCompany: %s\nDescription: %s\n", j.Title, j.Company, j.Description)
}

func generationSchema() map[string]any {
	stringProp := map[string]string{"type": "string"}

	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"cover_letter": stringProp,
			"tailored_bullets": map[string]any{
				"type":  "array",
				"items": stringProp,
			},
		},
		"required": []string{"cover_letter", "tailored_bullets"},
	}
}

func parseGenerationResponse(body []byte) (*GeneratedContent, error) {
	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode ollama response: %w", err)
	}

	if strings.TrimSpace(parsed.Message.Content) == "" {
		return nil, fmt.Errorf("ollama returned an empty response")
	}

	var generated GeneratedContent
	if err := json.Unmarshal([]byte(parsed.Message.Content), &generated); err != nil {
		return nil, fmt.Errorf("decode generated content: %w", err)
	}

	return &generated, nil
}
