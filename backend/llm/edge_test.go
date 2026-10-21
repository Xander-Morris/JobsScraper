package llm

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFormatResumeProfileProjects(t *testing.T) {
	got := formatResumeProfile(ResumeProfile{Projects: []ProjectEntry{
		{Name: "Crawler", Technologies: []string{"Go", "SQL"}, Bullets: []string{"Indexed jobs"}},
		{Name: "Game"},
	}})

	for _, want := range []string{"Project: Crawler (Go, SQL)\n- Indexed jobs", "Project: Game\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

// truncatedServer promises more body than it sends, so reading it fails.
func truncatedServer(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		fmt.Fprint(buf, "HTTP/1.1 200 OK\r\nContent-Length: 100\r\n\r\npartial")
		_ = buf.Flush()
		_ = conn.Close()
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestPostJSONTruncatedResponse(t *testing.T) {
	if _, _, err := postJSON(t.Context(), truncatedServer(t), "k", map[string]string{}); err == nil {
		t.Error("truncated body: want error")
	}
}

func TestCallsFailWhenUpstreamUnreachable(t *testing.T) {
	t.Setenv("JINA_API_KEY", "k")
	t.Setenv("JINA_API_URL", "http://127.0.0.1:1")
	t.Setenv("OPENROUTER_API_KEY", "k")
	t.Setenv("OPENROUTER_API_URL", "http://127.0.0.1:1")

	if _, err := callJinaEmbed(t.Context(), []string{"x"}); err == nil {
		t.Error("jina: want error")
	}
	if _, err := callOpenRouterChat(t.Context(), "p", map[string]any{}, 0); err == nil {
		t.Error("openrouter: want error")
	}
	if _, err := callOpenRouterChat(t.Context(), "p", map[string]any{"bad": make(chan int)}, 0); err == nil {
		t.Error("unencodable schema: want error")
	}
}

func TestTailorResumeUpstreamError(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")

	if _, err := TailorResume(t.Context(), testExtractedResume(), JobPosting{}); err == nil {
		t.Error("want error without an API key")
	}
}

func TestFormatIndexedResumeEducation(t *testing.T) {
	r := testExtractedResume()
	r.Education = []EducationEntry{{SchoolName: "MIT", Degree: "BS", Major: "CS"}}

	if got := formatIndexedResume(r); !strings.Contains(got, "Education: BS in CS, MIT") {
		t.Errorf("education missing: %q", got)
	}
}

func TestHydrateKeepsOriginalProjectBulletsWhenNoneValid(t *testing.T) {
	got := hydrateTailoredResume(testExtractedResume(), TailoringResult{Projects: []TailoredSection{{Source: "P0"}}})

	if len(got.Projects) != 1 || len(got.Projects[0].Bullets) != 1 || got.Projects[0].Bullets[0].Text != "Indexed jobs" {
		t.Errorf("Projects = %+v, want Crawler with its original bullet", got.Projects)
	}
}

func TestExtractResumeTextPDF(t *testing.T) {
	got, err := extractResumeText("application/pdf", buildPDF("Jane Doe", ""))
	if err != nil || !strings.Contains(got, "Jane Doe") {
		t.Errorf("extractResumeText(pdf) = %q, %v", got, err)
	}
}

// rawPDF builds a one-page PDF from a content stream and link annotations.
func rawPDF(stream string, links ...string) []byte {
	var annotRefs []string
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"", // page, filled in once annotation numbers are known
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
	}
	for _, link := range links {
		objects = append(objects, fmt.Sprintf("<< /Type /Annot /Subtype /Link /Rect [0 0 10 10] /A << /S /URI /URI (%s) >> >>", link))
		annotRefs = append(annotRefs, fmt.Sprintf("%d 0 R", len(objects)))
	}
	objects[2] = "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> /Annots [" +
		strings.Join(annotRefs, " ") + "] >>"

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

func TestExtractPdfLinksSkipsEmptyAndDuplicates(t *testing.T) {
	got, err := extractPdfText(rawPDF("BT /F1 12 Tf 72 720 Td (Jane) Tj ET", "https://a.dev", "", "https://a.dev"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(got, "https://a.dev") != 1 {
		t.Errorf("want one deduped link: %q", got)
	}
}

func TestExtractPdfTextBrokenContent(t *testing.T) {
	if _, err := extractPdfText(rawPDF("BT /F1 12 Tf Tj ET")); err == nil {
		t.Error("broken content stream: want error")
	}
}

// unreadableDocx zips parts with a compression method nothing can read, so opening them fails.
func unreadableDocx(t *testing.T, unreadable string, parts map[string]string) []byte {
	t.Helper()
	const bogusMethod = 99

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	zw.RegisterCompressor(bogusMethod, func(w io.Writer) (io.WriteCloser, error) { return nopWriteCloser{w}, nil })

	for name, body := range parts {
		method := zip.Store
		if name == unreadable {
			method = bogusMethod
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func TestExtractDocxUnreadableParts(t *testing.T) {
	if _, err := extractDocxText(unreadableDocx(t, "word/document.xml", map[string]string{"word/document.xml": testDocumentXML})); err == nil {
		t.Error("unreadable document.xml: want error")
	}

	got, err := extractDocxText(unreadableDocx(t, "word/_rels/document.xml.rels", map[string]string{
		"word/document.xml":            testDocumentXML,
		"word/_rels/document.xml.rels": testRelsXML,
	}))
	if err != nil || strings.Contains(got, "Hyperlinks") {
		t.Errorf("unreadable rels should just drop links: %q, %v", got, err)
	}
}

func TestExtractDocxBadTextRun(t *testing.T) {
	doc := `<w:document xmlns:w="w"><w:body><w:p><w:r><w:t>a<b></w:t></w:r></w:p></w:body></w:document>`
	if _, err := extractDocxText(buildDocx(t, map[string]string{"word/document.xml": doc})); err == nil {
		t.Error("malformed text run: want error")
	}
}

func TestFormatJobPostingTags(t *testing.T) {
	if got := formatJobPosting(JobPosting{Tags: []string{"go", "sql"}}); !strings.Contains(got, "Tags: go, sql\n") {
		t.Errorf("tags missing: %q", got)
	}
}

func TestGenerateTriviaUpstreamError(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")

	if _, err := GenerateTrivia(t.Context(), TriviaRequest{Prompt: "Go", Count: 5}); err == nil {
		t.Error("want error without an API key")
	}
}
