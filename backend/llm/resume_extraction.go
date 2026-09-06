package llm

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

var ErrUnsupportedFormat = fmt.Errorf("resume format is not supported for extraction")

type EducationEntry struct {
	SchoolName string `json:"school_name"`
	Degree     string `json:"degree"`
	Major      string `json:"major"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
}

type WorkExperienceEntry struct {
	Company   string   `json:"company"`
	JobTitle  string   `json:"job_title"`
	Location  string   `json:"location"`
	StartDate string   `json:"start_date"`
	EndDate   string   `json:"end_date"`
	Bullets   []string `json:"bullets"`
}

type ProjectEntry struct {
	Name         string   `json:"name"`
	Url          string   `json:"url"`
	Technologies []string `json:"technologies"`
	Bullets      []string `json:"bullets"`
}

type ExtractedResume struct {
	FullName       string                `json:"full_name"`
	Email          string                `json:"email"`
	Phone          string                `json:"phone"`
	LinkedIn       string                `json:"linked_in"`
	GitHub         string                `json:"github"`
	Portfolio      string                `json:"portfolio"`
	Summary        string                `json:"summary"`
	Skills         []string              `json:"skills"`
	Education      []EducationEntry      `json:"education"`
	WorkExperience []WorkExperienceEntry `json:"work_experience"`
	Projects       []ProjectEntry        `json:"projects"`
}

func ExtractResumeFields(ctx context.Context, fileName, contentType string, content []byte) (*ExtractedResume, error) {
	text, err := extractResumeText(contentType, content)
	if err != nil {
		return nil, err
	}

	respText, err := callGeminiChat(ctx, extractionPrompt+"\n\nResume text:\n"+text, resumeSchema())
	if err != nil {
		return nil, err
	}

	return parseExtractionResponse(respText)
}

func extractResumeText(contentType string, content []byte) (string, error) {
	switch contentType {
	case "application/pdf":
		return extractPdfText(content)
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return extractDocxText(content)
	default:
		return "", ErrUnsupportedFormat
	}
}

const extractionPrompt = "Extract structured fields from the resume text below as JSON matching the given schema. " +
	"Use an empty string for any field you cannot find, and an empty array for missing lists. " +
	"Do not invent information that isn't in the resume. " +
	"For URL fields, only use a URL that appears verbatim in the text (including any 'Hyperlinks embedded in this document' " +
	"list) — never guess or construct one from a project/company name. " +
	"The header/contact area often has several distinct links next to the name and email (e.g. a personal " +
	"website/portfolio, a GitHub profile, and a LinkedIn profile) — treat each as a separate field (linked_in, " +
	"github, portfolio) rather than collapsing them into one. " +
	"All start_date and end_date fields must be formatted as YYYY-MM-DD. If only a month and year are given, use " +
	"the first day of that month (e.g. 'Aug 2022' becomes '2022-08-01'); if only a year is given, use January 1st " +
	"of that year. If the entry is ongoing (e.g. 'Present' or 'Current'), leave end_date as an empty string."

func resumeSchema() map[string]any {
	stringProp := map[string]string{"type": "string"}

	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"full_name": stringProp,
			"email":     stringProp,
			"phone":     stringProp,
			"linked_in": stringProp,
			"github":    stringProp,
			"portfolio": stringProp,
			"summary":   stringProp,
			"skills": map[string]any{
				"type":  "array",
				"items": stringProp,
			},
			"education": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"school_name": stringProp,
						"degree":      stringProp,
						"major":       stringProp,
						"start_date":  stringProp,
						"end_date":    stringProp,
					},
					"required": []string{"school_name", "degree", "major", "start_date", "end_date"},
				},
			},
			"work_experience": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"company":    stringProp,
						"job_title":  stringProp,
						"location":   stringProp,
						"start_date": stringProp,
						"end_date":   stringProp,
						"bullets": map[string]any{
							"type":  "array",
							"items": stringProp,
						},
					},
					"required": []string{"company", "job_title", "location", "start_date", "end_date", "bullets"},
				},
			},
			"projects": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": stringProp,
						"url":  stringProp,
						"technologies": map[string]any{
							"type":  "array",
							"items": stringProp,
						},
						"bullets": map[string]any{
							"type":  "array",
							"items": stringProp,
						},
					},
					"required": []string{"name", "url", "technologies", "bullets"},
				},
			},
		},
		"required": []string{"full_name", "email", "phone", "linked_in", "github", "portfolio", "summary", "skills", "education", "work_experience", "projects"},
	}
}

func parseExtractionResponse(text string) (*ExtractedResume, error) {
	var extracted ExtractedResume
	if err := json.Unmarshal([]byte(text), &extracted); err != nil {
		return nil, fmt.Errorf("decode extracted fields: %w", err)
	}

	return &extracted, nil
}

func extractPdfText(content []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("read pdf: %w", err)
	}

	textReader, err := reader.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("extract pdf text: %w", err)
	}

	var sb strings.Builder
	if _, err := io.Copy(&sb, textReader); err != nil {
		return "", fmt.Errorf("read pdf text: %w", err)
	}

	if strings.TrimSpace(sb.String()) == "" {
		return "", fmt.Errorf("%w: no extractable text (scanned/image-only PDF)", ErrUnsupportedFormat)
	}

	appendLinks(&sb, extractPdfLinks(reader))

	return sb.String(), nil
}

func extractPdfLinks(reader *pdf.Reader) []string {
	seen := make(map[string]bool)
	var links []string

	for i := 1; i <= reader.NumPage(); i++ {
		annots := reader.Page(i).V.Key("Annots")
		for j := 0; j < annots.Len(); j++ {
			uri := annots.Index(j).Key("A").Key("URI").Text()
			if uri == "" || seen[uri] {
				continue
			}

			seen[uri] = true
			links = append(links, uri)
		}
	}

	return links
}

func appendLinks(sb *strings.Builder, links []string) {
	if len(links) == 0 {
		return
	}

	sb.WriteString("\n\nHyperlinks embedded in this document:\n")
	for _, link := range links {
		sb.WriteString("- ")
		sb.WriteString(link)
		sb.WriteString("\n")
	}
}

func extractDocxText(content []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("read docx zip: %w", err)
	}

	var docFile *zip.File
	for _, f := range reader.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return "", fmt.Errorf("word/document.xml not found in docx")
	}

	rc, err := docFile.Open()
	if err != nil {
		return "", fmt.Errorf("open document.xml: %w", err)
	}
	defer rc.Close()

	var sb strings.Builder
	decoder := xml.NewDecoder(rc)
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("parse document.xml: %w", err)
		}

		switch el := tok.(type) {
		case xml.StartElement:
			if el.Name.Local == "t" {
				var text string
				if err := decoder.DecodeElement(&text, &el); err != nil {
					return "", fmt.Errorf("decode text run: %w", err)
				}
				sb.WriteString(text)
			}
		case xml.EndElement:
			if el.Name.Local == "p" {
				sb.WriteString("\n")
			}
		}
	}

	appendLinks(&sb, extractDocxLinks(reader))

	return sb.String(), nil
}

func extractDocxLinks(reader *zip.Reader) []string {
	var relsFile *zip.File
	for _, f := range reader.File {
		if f.Name == "word/_rels/document.xml.rels" {
			relsFile = f
			break
		}
	}
	if relsFile == nil {
		return nil
	}

	rc, err := relsFile.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()

	var rels struct {
		Relationship []struct {
			Type       string `xml:"Type,attr"`
			Target     string `xml:"Target,attr"`
			TargetMode string `xml:"TargetMode,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.NewDecoder(rc).Decode(&rels); err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var links []string
	for _, rel := range rels.Relationship {
		if rel.TargetMode != "External" || !strings.HasSuffix(rel.Type, "/hyperlink") || seen[rel.Target] {
			continue
		}

		seen[rel.Target] = true
		links = append(links, rel.Target)
	}

	return links
}
