package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"main/database"
)

// testDocx builds a minimal .docx whose text is body.
func testDocx(t *testing.T, body string) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}

	fmt.Fprintf(w, `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>%s</w:t></w:r></w:p></w:body></w:document>`, body)

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

const extractionJSON = `{"full_name":"Ada Lovelace","email":"ada@example.com","phone":"","linked_in":"","github":"",
	"portfolio":"","summary":"Engineer","skills":["Go","Kubernetes"],"education":[],
	"work_experience":[{"company":"Acme","job_title":"Dev","location":"","start_date":"2021-01-01","end_date":"","bullets":["Built APIs"]}],
	"projects":[]}`

func TestAPIResumeLifecycle(t *testing.T) {
	a := newAPI(t).signup("api-resume@example.com")
	other := a.as().signup("api-resume-other@example.com")
	llm := fakeLLM(t, static(extractionJSON))

	content := testDocx(t, "Ada Lovelace, Go engineer")

	status, raw := a.upload("POST", "/api/profile/resumes", "ada.docx", content, nil)
	if status != http.StatusCreated {
		t.Fatalf("upload = %d: %s", status, raw)
	}

	resumes := a.profile().Resumes
	if len(resumes) != 1 || !resumes[0].IsActive || resumes[0].FileName != "ada.docx" ||
		resumes[0].ContentType != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Fatalf("resumes = %+v", resumes)
	}
	id := resumes[0].ID
	base := fmt.Sprintf("/api/profile/resumes/%d", id)

	var extraction database.ResumeExtraction
	a.must(http.StatusOK, "GET", base+"/extraction", nil, &extraction)
	if extraction.Status != "pending" {
		t.Errorf("extraction after upload = %q, want pending", extraction.Status)
	}

	var trigger map[string]string
	a.must(http.StatusOK, "POST", base+"/extraction", nil, &trigger)
	if trigger["status"] != "completed" {
		t.Fatalf("trigger = %v", trigger)
	}

	if llm.count("chat") != 1 || llm.count("embed") != 1 {
		t.Errorf("llm calls chat=%d embed=%d, want 1 each", llm.count("chat"), llm.count("embed"))
	}

	a.must(http.StatusOK, "GET", base+"/extraction", nil, &extraction)
	if extraction.Status != "completed" || extraction.FullName != "Ada Lovelace" || len(extraction.Skills) != 2 {
		t.Errorf("extraction = %+v", extraction)
	}

	// Download returns the original bytes as an attachment.
	req, _ := http.NewRequest("GET", a.srv.URL+base+"/download", nil)
	status, body, header := a.send(req)
	if status != http.StatusOK || !bytes.Equal(body, content) ||
		!strings.Contains(header.Get("Content-Disposition"), `filename=ada.docx`) {
		t.Errorf("download = %d, %d bytes, %q", status, len(body), header.Get("Content-Disposition"))
	}

	// Rename only.
	if status, raw := a.upload("PUT", base, "", nil, map[string]string{"file_name": "renamed.pdf"}); status != http.StatusOK {
		t.Fatalf("rename = %d: %s", status, raw)
	}
	if got := a.profile().Resumes[0].FileName; got != "renamed.pdf" {
		t.Errorf("file name = %q", got)
	}

	// Replace resets the extraction to pending.
	if status, raw := a.upload("PUT", base, "new.docx", testDocx(t, "v2"), nil); status != http.StatusOK {
		t.Fatalf("replace = %d: %s", status, raw)
	}
	a.must(http.StatusOK, "GET", base+"/extraction", nil, &extraction)
	if extraction.Status != "pending" {
		t.Errorf("extraction after replace = %q", extraction.Status)
	}

	// A second resume can be activated.
	status, raw = a.upload("POST", "/api/profile/resumes", "second.pdf", []byte("%PDF-fake"), nil)
	if status != http.StatusCreated {
		t.Fatalf("second upload = %d: %s", status, raw)
	}
	var second idResponse
	_ = json.Unmarshal(raw, &second)

	a.must(http.StatusOK, "POST", fmt.Sprintf("/api/profile/resumes/%d/activate", second.ID), nil, nil)
	for _, r := range a.profile().Resumes {
		if r.IsActive != (r.ID == second.ID) {
			t.Errorf("resume %d active = %v", r.ID, r.IsActive)
		}
	}

	// Other profiles see nothing.
	other.must(http.StatusNotFound, "GET", base+"/extraction", nil, nil)
	other.must(http.StatusNotFound, "POST", base+"/extraction", nil, nil)
	other.must(http.StatusNotFound, "POST", base+"/activate", nil, nil)
	other.must(http.StatusNotFound, "GET", base+"/download", nil, nil)
	other.must(http.StatusNotFound, "DELETE", base, nil, nil)
	if status, _ := other.upload("PUT", base, "", nil, map[string]string{"file_name": "x.pdf"}); status != http.StatusNotFound {
		t.Errorf("other rename = %d", status)
	}

	a.must(http.StatusOK, "DELETE", base, nil, nil)
	a.must(http.StatusNotFound, "DELETE", base, nil, nil)
}

func TestAPIResumeExtractionFailures(t *testing.T) {
	a := newAPI(t).signup("api-resume-fail@example.com")

	// A .doc can't be parsed: unsupported, and no LLM call.
	llm := fakeLLM(t, static(extractionJSON))
	status, raw := a.upload("POST", "/api/profile/resumes", "old.doc", []byte("binary"), nil)
	if status != http.StatusCreated {
		t.Fatalf("upload = %d: %s", status, raw)
	}
	var doc idResponse
	_ = json.Unmarshal(raw, &doc)

	var trigger map[string]string
	a.must(http.StatusOK, "POST", fmt.Sprintf("/api/profile/resumes/%d/extraction", doc.ID), nil, &trigger)
	if trigger["status"] != "unsupported" || llm.count("chat") != 0 {
		t.Errorf("trigger = %v, chat calls = %d", trigger, llm.count("chat"))
	}

	// Upstream failure records a failed extraction.
	fakeLLM(t, static(""))
	status, raw = a.upload("POST", "/api/profile/resumes", "r.docx", testDocx(t, "text"), nil)
	if status != http.StatusCreated {
		t.Fatalf("upload = %d: %s", status, raw)
	}
	var docx idResponse
	_ = json.Unmarshal(raw, &docx)

	path := fmt.Sprintf("/api/profile/resumes/%d/extraction", docx.ID)
	a.must(http.StatusOK, "POST", path, nil, &trigger)
	if trigger["status"] != "failed" {
		t.Errorf("trigger = %v, want failed", trigger)
	}

	var extraction database.ResumeExtraction
	a.must(http.StatusOK, "GET", path, nil, &extraction)
	if extraction.Status != "failed" || extraction.Error == "" {
		t.Errorf("extraction = %+v", extraction)
	}
}

func TestAPIResumeUploadValidation(t *testing.T) {
	a := newAPI(t).signup("api-resume-validate@example.com")

	cases := []struct {
		name     string
		method   string
		path     string
		fileName string
		content  []byte
		fields   map[string]string
		want     int
	}{
		{"no file", "POST", "/api/profile/resumes", "", nil, nil, http.StatusBadRequest},
		{"bad extension", "POST", "/api/profile/resumes", "resume.txt", []byte("x"), nil, http.StatusBadRequest},
		{"too big", "POST", "/api/profile/resumes", "big.pdf", bytes.Repeat([]byte("a"), maxResumeSize+1), nil, http.StatusBadRequest},
		{"bad id", "PUT", "/api/profile/resumes/x", "", nil, map[string]string{"file_name": "a.pdf"}, http.StatusBadRequest},
		{"bad rename", "PUT", "/api/profile/resumes/1", "", nil, map[string]string{"file_name": "a.exe"}, http.StatusBadRequest},
		{"missing rename", "PUT", "/api/profile/resumes/999999", "", nil, map[string]string{"file_name": "a.pdf"}, http.StatusNotFound},
		{"missing replace", "PUT", "/api/profile/resumes/999999", "a.pdf", []byte("x"), nil, http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if status, raw := a.upload(tc.method, tc.path, tc.fileName, tc.content, tc.fields); status != tc.want {
				t.Errorf("status = %d, want %d: %s", status, tc.want, raw)
			}
		})
	}

	// A non-multipart body is rejected.
	a.must(http.StatusBadRequest, "POST", "/api/profile/resumes", `{"resume":"x"}`, nil)

	for _, path := range []string{"/download", "/extraction", "/activate"} {
		method := "GET"
		if path == "/activate" {
			method = "POST"
		}
		a.must(http.StatusBadRequest, method, "/api/profile/resumes/x"+path, nil, nil)
	}
	a.must(http.StatusBadRequest, "POST", "/api/profile/resumes/x/extraction", nil, nil)
	a.must(http.StatusBadRequest, "DELETE", "/api/profile/resumes/x", nil, nil)
}

func TestNormalizeResumeName(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"resume.pdf", "resume.pdf", false},
		{"  Resume.PDF  ", "Resume.PDF", false},
		{"cv.docx", "cv.docx", false},
		{"cv.doc", "cv.doc", false},
		{"../../etc/resume.pdf", "resume.pdf", false},
		{"", "", true},
		{"   ", "", true},
		{"notes.txt", "", true},
		{"pdf", "", true},
		{strings.Repeat("a", 252) + ".pdf", "", true},
	}

	for _, tt := range tests {
		got, err := normalizeResumeName(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("normalizeResumeName(%q) = %q, %v", tt.in, got, err)
		}
	}
}

func TestContentTypeForResume(t *testing.T) {
	tests := map[string]string{
		"a.pdf":  "application/pdf",
		"a.PDF":  "application/pdf",
		"a.doc":  "application/msword",
		"a.docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	}

	for in, want := range tests {
		if got := contentTypeForResume(in); got != want {
			t.Errorf("contentTypeForResume(%q) = %q, want %q", in, got, want)
		}
	}
}
