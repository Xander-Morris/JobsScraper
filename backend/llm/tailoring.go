package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type TailoredBullet struct {
	Text string `json:"text"`
	// Source is the original bullet id (e.g. "W0.B2"), empty if added in the editor.
	Source string `json:"source"`
}

type TailoredWorkExperience struct {
	Company   string           `json:"company"`
	JobTitle  string           `json:"job_title"`
	Location  string           `json:"location"`
	StartDate string           `json:"start_date"`
	EndDate   string           `json:"end_date"`
	Bullets   []TailoredBullet `json:"bullets"`
}

type TailoredProject struct {
	Name         string           `json:"name"`
	Url          string           `json:"url"`
	Technologies []string         `json:"technologies"`
	Bullets      []TailoredBullet `json:"bullets"`
}

type TailoredResume struct {
	FullName       string                   `json:"full_name"`
	Email          string                   `json:"email"`
	Phone          string                   `json:"phone"`
	LinkedIn       string                   `json:"linked_in"`
	GitHub         string                   `json:"github"`
	Portfolio      string                   `json:"portfolio"`
	Summary        string                   `json:"summary"`
	Skills         []string                 `json:"skills"`
	Education      []EducationEntry         `json:"education"`
	WorkExperience []TailoredWorkExperience `json:"work_experience"`
	Projects       []TailoredProject        `json:"projects"`
}

// TailoringResult is the raw model output: wording and source ids only, no facts.
type TailoringResult struct {
	Summary        string            `json:"summary"`
	Skills         []string          `json:"skills"`
	WorkExperience []TailoredSection `json:"work_experience"`
	Projects       []TailoredSection `json:"projects"`
}

type TailoredSection struct {
	Source  string           `json:"source"`
	Bullets []TailoredBullet `json:"bullets"`
}

const tailoringPrompt = "Tailor the candidate's resume below to the job posting that follows. Every resume item is " +
	"labelled with an id in square brackets: roles are [W0], [W1], ..., projects are [P0], [P1], ..., and bullets are " +
	"[W0.B0], [P1.B2], and so on. Return only rewritten text plus the ids it came from. Never return company names, " +
	"job titles, dates, or schools. Base every claim only on the resume: do not invent skills, tools, employers, " +
	"metrics, or accomplishments. Rewrite each bullet you keep so it uses the posting's terminology where that is " +
	"truthful, keeping the candidate's real numbers, and set its source to the id of the single original bullet it " +
	"was rewritten from. Include every role, ordering each role's bullets from most to least relevant to this job " +
	"and dropping irrelevant ones, but keep at least one bullet per role. Include only the projects relevant to this " +
	"job, most relevant first. List skills from the resume's own skills list only, most relevant to this job first, " +
	"dropping ones that don't matter for it. Write a 2-3 sentence summary aimed at this role, grounded only in the resume."

func TailorResume(ctx context.Context, resume ExtractedResume, job JobPosting) (*TailoredResume, error) {
	prompt := tailoringPrompt + "\n\n" + formatIndexedResume(resume) + "\n\n" + formatJobPosting(job)

	respText, err := callOpenRouterChat(ctx, prompt, tailoringSchema())
	if err != nil {
		return nil, err
	}

	var result TailoringResult
	if err := json.Unmarshal([]byte(respText), &result); err != nil {
		return nil, fmt.Errorf("decode tailored resume: %w", err)
	}

	tailored := hydrateTailoredResume(resume, result)

	return &tailored, nil
}

func formatIndexedResume(r ExtractedResume) string {
	var sb strings.Builder

	sb.WriteString("Candidate resume:\n")
	if r.Summary != "" {
		fmt.Fprintf(&sb, "Summary: %s\n", r.Summary)
	}
	if len(r.Skills) > 0 {
		fmt.Fprintf(&sb, "Skills: %s\n", strings.Join(r.Skills, ", "))
	}
	for _, edu := range r.Education {
		fmt.Fprintf(&sb, "Education: %s in %s, %s\n", edu.Degree, edu.Major, edu.SchoolName)
	}

	for i, we := range r.WorkExperience {
		fmt.Fprintf(&sb, "\n[W%d] Role: %s at %s (%s to %s)\n", i, we.JobTitle, we.Company, we.StartDate, orPresent(we.EndDate))
		for j, bullet := range we.Bullets {
			fmt.Fprintf(&sb, "[W%d.B%d] %s\n", i, j, bullet)
		}
	}

	for i, proj := range r.Projects {
		fmt.Fprintf(&sb, "\n[P%d] Project: %s", i, proj.Name)
		if len(proj.Technologies) > 0 {
			fmt.Fprintf(&sb, " (%s)", strings.Join(proj.Technologies, ", "))
		}
		sb.WriteString("\n")
		for j, bullet := range proj.Bullets {
			fmt.Fprintf(&sb, "[P%d.B%d] %s\n", i, j, bullet)
		}
	}

	return sb.String()
}

func tailoringSchema() map[string]any {
	stringProp := map[string]string{"type": "string"}

	sections := map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"source": stringProp,
				"bullets": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"text":   stringProp,
							"source": stringProp,
						},
						"required": []string{"text", "source"},
					},
				},
			},
			"required": []string{"source", "bullets"},
		},
	}

	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"summary": stringProp,
			"skills": map[string]any{
				"type":  "array",
				"items": stringProp,
			},
			"work_experience": sections,
			"projects":        sections,
		},
		"required": []string{"summary", "skills", "work_experience", "projects"},
	}
}

// hydrateTailoredResume takes wording and order from the model, everything else from source.
func hydrateTailoredResume(source ExtractedResume, result TailoringResult) TailoredResume {
	tailored := TailoredResume{
		FullName:       source.FullName,
		Email:          source.Email,
		Phone:          source.Phone,
		LinkedIn:       source.LinkedIn,
		GitHub:         source.GitHub,
		Portfolio:      source.Portfolio,
		Summary:        strings.TrimSpace(result.Summary),
		Skills:         tailoredSkills(source.Skills, result.Skills),
		Education:      nonNil(source.Education),
		WorkExperience: []TailoredWorkExperience{},
		Projects:       []TailoredProject{},
	}

	if tailored.Summary == "" {
		tailored.Summary = source.Summary
	}

	roleBullets := make([][]string, len(source.WorkExperience))
	for i, we := range source.WorkExperience {
		roleBullets[i] = we.Bullets
	}

	byRole := make(map[int][]TailoredBullet)
	for _, section := range validSections(result.WorkExperience, "W", roleBullets) {
		byRole[section.index] = section.bullets
	}

	// Every role is kept, in original order, so there are no timeline gaps.
	for i, we := range source.WorkExperience {
		bullets := byRole[i]
		if len(bullets) == 0 {
			bullets = originalBullets("W", i, we.Bullets)
		}

		tailored.WorkExperience = append(tailored.WorkExperience, TailoredWorkExperience{
			Company:   we.Company,
			JobTitle:  we.JobTitle,
			Location:  we.Location,
			StartDate: we.StartDate,
			EndDate:   we.EndDate,
			Bullets:   bullets,
		})
	}

	projectBullets := make([][]string, len(source.Projects))
	for i, proj := range source.Projects {
		projectBullets[i] = proj.Bullets
	}

	for _, section := range validSections(result.Projects, "P", projectBullets) {
		proj := source.Projects[section.index]
		bullets := section.bullets
		if len(bullets) == 0 {
			bullets = originalBullets("P", section.index, proj.Bullets)
		}

		tailored.Projects = append(tailored.Projects, TailoredProject{
			Name:         proj.Name,
			Url:          proj.Url,
			Technologies: nonNil(proj.Technologies),
			Bullets:      bullets,
		})
	}

	return tailored
}

type sectionBullets struct {
	index   int
	bullets []TailoredBullet
}

func validSections(sections []TailoredSection, prefix string, originals [][]string) []sectionBullets {
	seen := make(map[int]bool)
	var out []sectionBullets

	for _, section := range sections {
		index, bullet, ok := parseSourceID(section.Source, prefix)
		if !ok || bullet != -1 || index >= len(originals) || seen[index] {
			continue
		}
		seen[index] = true

		kept := []TailoredBullet{}
		for _, b := range section.Bullets {
			text := strings.TrimSpace(b.Text)
			srcIndex, srcBullet, ok := parseSourceID(b.Source, prefix)
			if text == "" || !ok || srcIndex != index || srcBullet < 0 || srcBullet >= len(originals[index]) {
				continue
			}

			kept = append(kept, TailoredBullet{Text: text, Source: bulletID(prefix, index, srcBullet)})
		}

		out = append(out, sectionBullets{index: index, bullets: kept})
	}

	return out
}

func originalBullets(prefix string, index int, bullets []string) []TailoredBullet {
	out := make([]TailoredBullet, 0, len(bullets))

	for j, bullet := range bullets {
		out = append(out, TailoredBullet{Text: bullet, Source: bulletID(prefix, index, j)})
	}

	return out
}

func bulletID(prefix string, index, bullet int) string {
	return fmt.Sprintf("%s%d.B%d", prefix, index, bullet)
}

// parseSourceID parses "W3" (bullet -1) or "W3.B1".
func parseSourceID(id, prefix string) (section, bullet int, ok bool) {
	id = strings.Trim(strings.TrimSpace(id), "[]")
	sectionPart, bulletPart, hasBullet := strings.Cut(id, ".")

	section, ok = parseIndex(sectionPart, prefix)
	if !ok {
		return 0, 0, false
	}

	if !hasBullet {
		return section, -1, true
	}

	bullet, ok = parseIndex(bulletPart, "B")
	if !ok {
		return 0, 0, false
	}

	return section, bullet, true
}

func parseIndex(s, prefix string) (int, bool) {
	digits, found := strings.CutPrefix(s, prefix)
	if !found || digits == "" {
		return 0, false
	}

	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 {
		return 0, false
	}

	return n, true
}

func tailoredSkills(original, chosen []string) []string {
	canonical := make(map[string]string, len(original))
	for _, skill := range original {
		key := strings.ToLower(strings.TrimSpace(skill))
		if _, exists := canonical[key]; key != "" && !exists {
			canonical[key] = skill
		}
	}

	seen := make(map[string]bool)
	out := []string{}

	for _, skill := range chosen {
		key := strings.ToLower(strings.TrimSpace(skill))
		spelled, ok := canonical[key]
		if !ok || seen[key] {
			continue
		}

		seen[key] = true
		out = append(out, spelled)
	}

	if len(out) == 0 {
		return nonNil(original)
	}

	return out
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}

	return s
}
