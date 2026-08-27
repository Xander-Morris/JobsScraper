package llm

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	anthropicAPIURL     = "https://api.anthropic.com/v1/messages"
	anthropicVersion    = "2023-06-01"
	defaultResumeModel  = "claude-haiku-4-5-20251001"
	extractionToolName  = "extract_resume"
	extractionMaxTokens = 4096
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

type ExtractedResume struct {
	FullName       string                `json:"full_name"`
	Email          string                `json:"email"`
	Phone          string                `json:"phone"`
	Summary        string                `json:"summary"`
	Skills         []string              `json:"skills"`
	Education      []EducationEntry      `json:"education"`
	WorkExperience []WorkExperienceEntry `json:"work_experience"`
}

func resumeModel() string {
	if model := os.Getenv("ANTHROPIC_MODEL"); model != "" {
		return model
	}

	return defaultResumeModel
}

func ExtractResumeFields(ctx context.Context, fileName, contentType string, content []byte) (*ExtractedResume, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is not configured")
	}

	userContent, err := buildUserContent(fileName, contentType, content)
	if err != nil {
		return nil, err
	}

	reqBody := map[string]any{
		"model":      resumeModel(),
		"max_tokens": extractionMaxTokens,
		"tools":      []any{extractionTool()},
		"tool_choice": map[string]string{
			"type": "tool",
			"name": extractionToolName,
		},
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": userContent,
			},
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call anthropic api: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read anthropic response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic api returned %d: %s", resp.StatusCode, string(respBody))
	}

	return parseExtractionResponse(respBody)
}

func buildUserContent(fileName, contentType string, content []byte) ([]any, error) {
	switch {
	case contentType == "application/pdf":
		return []any{
			map[string]any{
				"type": "document",
				"source": map[string]string{
					"type":       "base64",
					"media_type": "application/pdf",
					"data":       base64.StdEncoding.EncodeToString(content),
				},
			},
			map[string]any{
				"type": "text",
				"text": extractionPrompt,
			},
		}, nil
	case contentType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		text, err := extractDocxText(content)
		if err != nil {
			return nil, fmt.Errorf("extract docx text: %w", err)
		}

		return []any{
			map[string]any{
				"type": "text",
				"text": extractionPrompt + "\n\nResume text:\n" + text,
			},
		}, nil
	default:
		return nil, ErrUnsupportedFormat
	}
}

const extractionPrompt = "Extract structured fields from the attached resume by calling the extract_resume tool. " +
	"Use an empty string for any field you cannot find, and an empty array for missing lists. " +
	"Do not invent information that isn't in the resume."

func extractionTool() map[string]any {
	stringProp := map[string]string{"type": "string"}

	return map[string]any{
		"name":        extractionToolName,
		"description": "Record the structured fields extracted from a resume.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"full_name": stringProp,
				"email":     stringProp,
				"phone":     stringProp,
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
			},
			"required": []string{"full_name", "email", "phone", "summary", "skills", "education", "work_experience"},
		},
	}
}

type messagesResponse struct {
	Content []struct {
		Type  string          `json:"type"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content"`
}

func parseExtractionResponse(body []byte) (*ExtractedResume, error) {
	var parsed messagesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode anthropic response: %w", err)
	}

	for _, block := range parsed.Content {
		if block.Type != "tool_use" || block.Name != extractionToolName {
			continue
		}

		var extracted ExtractedResume
		if err := json.Unmarshal(block.Input, &extracted); err != nil {
			return nil, fmt.Errorf("decode tool input: %w", err)
		}

		return &extracted, nil
	}

	return nil, fmt.Errorf("anthropic response did not include a tool_use block")
}

// extractDocxText pulls all visible text out of a .docx file's word/document.xml by
// reading every <w:t> run, using only the standard library (docx is a zip of XML).
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

	return sb.String(), nil
}
