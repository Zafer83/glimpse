/*
Copyright 2026 Zafer Kılıçaslan

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package crawler

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// extractDocxText reads a .docx file and returns the plain text content.
// DOCX files are ZIP archives containing word/document.xml with <w:t> text nodes.
func extractDocxText(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open docx: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open document.xml: %w", err)
		}
		defer rc.Close()
		return parseDocumentXML(rc)
	}
	return "", fmt.Errorf("word/document.xml not found in %s", path)
}

// parseDocumentXML extracts text from Word's document.xml by collecting all <w:t> elements.
// Inserts spaces between runs and newlines between paragraphs.
func parseDocumentXML(r io.Reader) (string, error) {
	decoder := xml.NewDecoder(r)
	var result strings.Builder
	var inText bool
	var paragraphHasText bool

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("parse document.xml: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "p":
				if paragraphHasText {
					result.WriteString("\n")
				}
				paragraphHasText = false
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
				result.WriteString(" ")
			}
		case xml.CharData:
			if inText {
				result.Write(t)
				paragraphHasText = true
			}
		}
	}
	return strings.TrimSpace(result.String()), nil
}

// extractPDFText extracts text from a PDF using the pdftotext command-line tool.
// Returns an error if pdftotext is not installed.
func extractPDFText(path string) (string, error) {
	bin, err := exec.LookPath("pdftotext")
	if err != nil {
		return "", fmt.Errorf("pdftotext not found: install poppler-utils to read PDF files")
	}

	out, err := exec.Command(bin, "-layout", path, "-").Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext failed for %s: %w", path, err)
	}
	return strings.TrimSpace(string(out)), nil
}
