package llm

import (
	"context"
	"encoding/json"
	"fmt"
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
	Tags        []string
	Description string
}

type GeneratedContent struct {
	CoverLetter     string   `json:"cover_letter"`
	TailoredBullets []string `json:"tailored_bullets"`
}

const generationPrompt = "Using the candidate's resume information and the job posting below, write a tailored " +
	"cover letter and a short list of resume bullet points reworded to highlight the experience most relevant to " +
	"this job. Base every claim only on the resume information given. Do not invent skills, employers, titles, " +
	"or accomplishments that aren't present in it. Keep the cover letter to 3-4 short paragraphs, specific to the " +
	"role and company rather than generic. Tailored bullets should reuse the candidate's real achievements and " +
	"numbers where available, rewritten to emphasize relevance to this job's requirements. Do not fabricate new ones."

func GenerateApplicationContent(ctx context.Context, profile ResumeProfile, job JobPosting) (*GeneratedContent, error) {
	prompt := generationPrompt + "\n\n" + formatResumeProfile(profile) + "\n\n" + formatJobPosting(job)

	respText, err := callOpenRouterChat(ctx, prompt, generationSchema())
	if err != nil {
		return nil, err
	}

	return parseGenerationResponse(respText)
}

// EmbeddingText renders a resume profile as the same compact plain text
// GenerateApplicationContent feeds the LLM, already a dense summary of the
// resume, so it's good material to embed too.
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
	var sb strings.Builder

	fmt.Fprintf(&sb, "Job posting:\nTitle: %s\nCompany: %s\n", j.Title, j.Company)
	if len(j.Tags) > 0 {
		fmt.Fprintf(&sb, "Tags: %s\n", strings.Join(j.Tags, ", "))
	}
	fmt.Fprintf(&sb, "Description: %s\n", j.Description)

	return sb.String()
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

func parseGenerationResponse(text string) (*GeneratedContent, error) {
	var generated GeneratedContent
	if err := json.Unmarshal([]byte(text), &generated); err != nil {
		return nil, fmt.Errorf("decode generated content: %w", err)
	}

	return &generated, nil
}
