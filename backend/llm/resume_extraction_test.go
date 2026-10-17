package llm

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const docxContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

// buildDocx zips the given parts into an in-memory .docx.
func buildDocx(t *testing.T, parts map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for name, body := range parts {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

const testDocumentXML = `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>
<w:p><w:r><w:t>Jane </w:t></w:r><w:r><w:t>Doe</w:t></w:r></w:p>
<w:p><w:r><w:t>Go &amp; Postgres</w:t></w:r></w:p>
</w:body>
</w:document>`

const testRelsXML = `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://github.com/jane" TargetMode="External"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://github.com/jane" TargetMode="External"/>
<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
<Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="#bookmark"/>
</Relationships>`

func TestExtractDocxText(t *testing.T) {
	content := buildDocx(t, map[string]string{
		"word/document.xml":            testDocumentXML,
		"word/_rels/document.xml.rels": testRelsXML,
	})

	got, err := extractDocxText(content)
	if err != nil {
		t.Fatal(err)
	}

	want := "Jane Doe\nGo & Postgres\n\n\nHyperlinks embedded in this document:\n- https://github.com/jane\n"
	if got != want {
		t.Errorf("extractDocxText() = %q, want %q", got, want)
	}
}

func TestExtractDocxTextWithoutRels(t *testing.T) {
	got, err := extractDocxText(buildDocx(t, map[string]string{"word/document.xml": testDocumentXML}))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(got, "Hyperlinks") {
		t.Errorf("no rels should mean no link list: %q", got)
	}
}

func TestExtractDocxTextErrors(t *testing.T) {
	if _, err := extractDocxText([]byte("not a zip")); err == nil {
		t.Error("non-zip: want error")
	}

	if _, err := extractDocxText(buildDocx(t, map[string]string{"other.xml": "<a/>"})); err == nil {
		t.Error("missing document.xml: want error")
	}

	if _, err := extractDocxText(buildDocx(t, map[string]string{"word/document.xml": "<w:document><w:p>"})); err == nil {
		t.Error("truncated xml: want error")
	}
}

func TestExtractDocxLinksBadRels(t *testing.T) {
	content := buildDocx(t, map[string]string{"word/_rels/document.xml.rels": "<<<"})

	reader, _ := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if links := extractDocxLinks(reader); links != nil {
		t.Errorf("links = %v, want nil", links)
	}
}

func TestAppendLinks(t *testing.T) {
	var sb strings.Builder
	appendLinks(&sb, nil)

	if sb.Len() != 0 {
		t.Errorf("no links should write nothing, got %q", sb.String())
	}

	appendLinks(&sb, []string{"a", "b"})
	if got := sb.String(); got != "\n\nHyperlinks embedded in this document:\n- a\n- b\n" {
		t.Errorf("appendLinks = %q", got)
	}
}

// buildPDF writes a one-page PDF showing text, with an optional link annotation.
func buildPDF(text, link string) []byte {
	stream := ""
	if text != "" {
		stream = fmt.Sprintf("BT /F1 12 Tf 72 720 Td (%s) Tj ET", text)
	}

	annots := ""
	if link != "" {
		annots = " /Annots [6 0 R]"
	}

	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >>" + annots + " >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
	}
	if link != "" {
		objects = append(objects, fmt.Sprintf("<< /Type /Annot /Subtype /Link /Rect [0 0 10 10] /A << /S /URI /URI (%s) >> >>", link))
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")

	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}

	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)

	return buf.Bytes()
}

func TestExtractPdfText(t *testing.T) {
	got, err := extractPdfText(buildPDF("Jane Doe Go Engineer", "https://jane.dev"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "Jane Doe Go Engineer") {
		t.Errorf("text missing: %q", got)
	}

	if !strings.Contains(got, "- https://jane.dev") {
		t.Errorf("link missing: %q", got)
	}
}

func TestExtractPdfTextImageOnly(t *testing.T) {
	_, err := extractPdfText(buildPDF("", ""))
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("err = %v, want ErrUnsupportedFormat", err)
	}
}

func TestExtractPdfTextInvalid(t *testing.T) {
	if _, err := extractPdfText([]byte("not a pdf")); err == nil {
		t.Error("want error")
	}
}

func TestExtractResumeTextUnsupported(t *testing.T) {
	if _, err := extractResumeText("text/plain", []byte("hi")); !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("err = %v, want ErrUnsupportedFormat", err)
	}
}

func TestExtractResumeFields(t *testing.T) {
	reqs := fakeOpenRouter(t, `{"full_name":"Jane Doe","skills":["Go"],"education":[],"work_experience":[],"projects":[]}`)

	content := buildDocx(t, map[string]string{"word/document.xml": testDocumentXML})

	got, err := ExtractResumeFields(t.Context(), "resume.docx", docxContentType, content)
	if err != nil {
		t.Fatal(err)
	}

	if got.FullName != "Jane Doe" || len(got.Skills) != 1 {
		t.Errorf("extracted = %+v", got)
	}

	req := (*reqs)[0]
	if req.Temperature != 0 || !strings.Contains(req.Messages[0].Content, "Resume text:\nJane Doe") {
		t.Errorf("request = %+v", req)
	}
}

func TestExtractResumeFieldsErrors(t *testing.T) {
	fakeOpenRouter(t, `not json`)

	if _, err := ExtractResumeFields(t.Context(), "r.txt", "text/plain", nil); !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("unsupported err = %v", err)
	}

	content := buildDocx(t, map[string]string{"word/document.xml": testDocumentXML})
	if _, err := ExtractResumeFields(t.Context(), "r.docx", docxContentType, content); err == nil {
		t.Error("bad model json: want error")
	}

	fakeOpenRouterRaw(t, 500, func() string { return "down" })
	if _, err := ExtractResumeFields(t.Context(), "r.docx", docxContentType, content); err == nil {
		t.Error("upstream failure: want error")
	}
}

func TestExtractedResumeProfile(t *testing.T) {
	r := testExtractedResume()
	p := r.Profile()

	if p.FullName != r.FullName || p.Summary != r.Summary || len(p.Skills) != len(r.Skills) ||
		len(p.WorkExperience) != len(r.WorkExperience) || len(p.Projects) != len(r.Projects) {
		t.Errorf("Profile() = %+v", p)
	}
}

func TestParseResponses(t *testing.T) {
	if _, err := parseExtractionResponse("{"); err == nil {
		t.Error("parseExtractionResponse: want error")
	}

	if got, err := parseExtractionResponse(`{"email":"a@b.c"}`); err != nil || got.Email != "a@b.c" {
		t.Errorf("parseExtractionResponse = %+v, %v", got, err)
	}

	if _, err := parseGenerationResponse("{"); err == nil {
		t.Error("parseGenerationResponse: want error")
	}

	if got, err := parseGenerationResponse(`{"cover_letter":"x"}`); err != nil || got.CoverLetter != "x" {
		t.Errorf("parseGenerationResponse = %+v, %v", got, err)
	}
}
