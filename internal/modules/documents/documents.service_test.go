package documents

import "testing"

func TestValidatePDFHeader(t *testing.T) {
	if !ValidatePDFHeader([]byte("%PDF-1.7\nrest")) {
		t.Fatal("expected valid PDF header")
	}
	if ValidatePDFHeader([]byte("not a pdf")) {
		t.Fatal("expected invalid PDF header")
	}
}

func TestSanitizeDocumentFilename(t *testing.T) {
	got := sanitizeDocumentFilename(`..\unsafe folder/my report final!.pdf`)
	if got != "my_report_final_.pdf" {
		t.Fatalf("sanitized filename = %q", got)
	}

	got = sanitizeDocumentFilename("no-extension")
	if got != "no-extension.pdf" {
		t.Fatalf("sanitized filename = %q", got)
	}
}
