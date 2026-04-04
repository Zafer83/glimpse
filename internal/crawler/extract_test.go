package crawler

import (
	"archive/zip"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractDocxText(t *testing.T) {
	// Create a minimal .docx file (ZIP with word/document.xml).
	dir := t.TempDir()
	docxPath := filepath.Join(dir, "test.docx")

	f, err := os.Create(docxPath)
	if err != nil {
		t.Fatalf("create docx: %v", err)
	}

	w := zip.NewWriter(f)

	// Write word/document.xml with sample content.
	docXML := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r><w:t>Hello World</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:t>Second paragraph</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`

	fw, err := w.Create("word/document.xml")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	fw.Write([]byte(docXML))
	w.Close()
	f.Close()

	text, err := extractDocxText(docxPath)
	if err != nil {
		t.Fatalf("extractDocxText: %v", err)
	}

	if !strings.Contains(text, "Hello World") {
		t.Errorf("expected 'Hello World' in output, got: %q", text)
	}
	if !strings.Contains(text, "Second paragraph") {
		t.Errorf("expected 'Second paragraph' in output, got: %q", text)
	}
}

func TestExtractDocxText_MissingDocumentXML(t *testing.T) {
	dir := t.TempDir()
	docxPath := filepath.Join(dir, "empty.docx")

	f, err := os.Create(docxPath)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	w := zip.NewWriter(f)
	// Write a dummy file, not word/document.xml.
	fw, _ := w.Create("other.xml")
	fw.Write([]byte("<root/>"))
	w.Close()
	f.Close()

	_, err = extractDocxText(docxPath)
	if err == nil {
		t.Error("expected error for docx without word/document.xml")
	}
}

func TestExtractDocxText_InvalidZip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.docx")
	os.WriteFile(path, []byte("not a zip file"), 0644)

	_, err := extractDocxText(path)
	if err == nil {
		t.Error("expected error for invalid zip")
	}
}

func TestParseDocumentXML(t *testing.T) {
	input := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r><w:t>Line one</w:t></w:r>
      <w:r><w:t> continued</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:t>Line two</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`

	text, err := parseDocumentXML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseDocumentXML: %v", err)
	}
	if !strings.Contains(text, "Line one") {
		t.Errorf("expected 'Line one', got: %q", text)
	}
	if !strings.Contains(text, "Line two") {
		t.Errorf("expected 'Line two', got: %q", text)
	}
}

func TestParseDocumentXML_InvalidXML(t *testing.T) {
	_, err := parseDocumentXML(strings.NewReader("<<<not xml"))
	if err == nil {
		t.Error("expected error for invalid XML")
	}
}

// Ensure the xml import is used.
var _ = xml.Name{}
