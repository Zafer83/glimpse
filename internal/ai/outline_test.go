package ai

import (
	"strings"
	"testing"

	"github.com/Zafer83/glimpse/internal/crawler"
)

func TestExtractDocOutline_BasicStructure(t *testing.T) {
	docs := []crawler.FileEntry{
		{
			Path: "docs/README.md",
			Content: `# LegalMind AI

Private, On-Premise KI-Infrastruktur für Rechtsanwaltskanzleien.

## Das Problem

Anwälte verlieren täglich 40-60% ihrer Zeit für Routinetätigkeiten.

- 2-4 Stunden täglich für juristische Recherche
- 1-2 Stunden für Schriftsatzerstellung
- Keine On-Premise KI für deutsches Recht

## Die Lösung

- Multi-Agent-System mit 9 spezialisierten Agenten
- Vollständig On-Premise, kein Cloud-Datentransfer
- §43a BRAO konform durch Architektur

## Technischer Stack

- Python 3.11 mit FastAPI
- ChromaDB als Vektordatenbank
- LLaMA 3.3 70B als Primärmodell
`,
		},
		{
			Path: "docs/VISION.md",
			Content: `# Vision

## Wettbewerbsposition

- Einziges echtes On-Premise-System
- GwG/KYC-Modul vollständig integriert

## Geschäftsmodell & Pricing

- SaaS: 159-499 EUR/Nutzer/Monat
- On-Premise: 35.000-120.000 EUR einmalig
`,
		},
	}

	outline := extractDocOutline(docs)

	if outline.ProjectName != "LegalMind AI" {
		t.Errorf("ProjectName = %q, want %q", outline.ProjectName, "LegalMind AI")
	}

	if !strings.Contains(outline.Description, "KI-Infrastruktur") {
		t.Errorf("Description = %q, should contain 'KI-Infrastruktur'", outline.Description)
	}

	if len(outline.Sections) < 4 {
		t.Errorf("got %d sections, want at least 4", len(outline.Sections))
	}

	// Check that sections have bullets.
	problemFound := false
	for _, sec := range outline.Sections {
		if strings.Contains(strings.ToLower(sec.Title), "problem") {
			problemFound = true
			if len(sec.Bullets) < 2 {
				t.Errorf("Problem section has %d bullets, want at least 2", len(sec.Bullets))
			}
		}
	}
	if !problemFound {
		t.Error("no 'Problem' section found in outline")
	}
}

func TestExtractDocOutline_EmptyDocs(t *testing.T) {
	outline := extractDocOutline(nil)
	if outline.ProjectName != "" {
		t.Errorf("expected empty project name for nil docs, got %q", outline.ProjectName)
	}
	if len(outline.Sections) != 0 {
		t.Errorf("expected 0 sections for nil docs, got %d", len(outline.Sections))
	}
}

func TestExtractDocOutline_DeduplicatesSections(t *testing.T) {
	docs := []crawler.FileEntry{
		{Path: "README.md", Content: "# Proj\n\nA project.\n\n## Architecture\n\n- Component A\n"},
		{Path: "docs/ARCH.md", Content: "# Arch\n\n## Architecture\n\n- Component B\n"},
	}
	outline := extractDocOutline(docs)

	count := 0
	for _, sec := range outline.Sections {
		if strings.ToLower(sec.Title) == "architecture" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("duplicate 'Architecture' sections: got %d, want 1", count)
	}
}

func TestExtractKeyStats(t *testing.T) {
	docs := []crawler.FileEntry{
		{
			Path: "docs/VISION.md",
			Content: `Anwälte verlieren 40-60% ihrer Zeit.
Preis: 159 EUR bis 499 EUR pro Nutzer.
Konformität mit §43a BRAO und §203 StGB.
ROI von 400-800%.`,
		},
	}

	stats := extractKeyStats(docs)
	if len(stats) < 3 {
		t.Errorf("got %d stats, want at least 3; stats=%v", len(stats), stats)
	}

	// Should find percentage patterns.
	found40 := false
	for _, s := range stats {
		if strings.Contains(s, "40") && strings.Contains(s, "%") {
			found40 = true
		}
	}
	if !found40 {
		t.Errorf("did not find '40-60%%' stat; stats=%v", stats)
	}
}

func TestMapSectionsToSlots(t *testing.T) {
	sections := []OutlineSection{
		{Title: "Das Problem", Bullets: []string{"bullet1"}},
		{Title: "Die Lösung", Bullets: []string{"bullet2"}},
		{Title: "Technischer Stack", Bullets: []string{"bullet3"}},
		{Title: "Compliance & DSGVO", Bullets: []string{"bullet4"}},
		{Title: "Random Topic", Bullets: []string{"bullet5"}},
	}

	slots := mapSectionsToSlots(sections)

	if len(slots["problem"]) != 1 {
		t.Errorf("problem slot: got %d sections, want 1", len(slots["problem"]))
	}
	if len(slots["solution"]) != 1 {
		t.Errorf("solution slot: got %d sections, want 1", len(slots["solution"]))
	}
	if len(slots["security"]) != 1 {
		t.Errorf("security slot: got %d sections, want 1", len(slots["security"]))
	}

	// "Random Topic" should overflow into features.
	featCount := len(slots["features"])
	if featCount < 1 {
		t.Errorf("unmatched section should overflow to features, got %d", featCount)
	}
}

func TestBuildSlidePlan_Local(t *testing.T) {
	outline := DocOutline{
		ProjectName: "TestProject",
		Description: "A test project for testing.",
		Sections: []OutlineSection{
			{Title: "Das Problem", Bullets: []string{"Too much manual work", "No automation"}},
			{Title: "Die Lösung", Bullets: []string{"AI-powered automation", "Cloud-native"}},
			{Title: "Architektur", Bullets: []string{"Microservices", "Event-driven"}},
		},
		KeyStats: []string{"40%", "100 EUR"},
	}

	plan := buildSlidePlan(outline, "de", true)

	// Must contain actual Slidev section divider markdown.
	if !strings.Contains(plan, "layout: section\n---") {
		t.Error("local slide plan should contain actual Slidev section frontmatter")
	}

	// Must contain pre-filled facts as bullets.
	if !strings.Contains(plan, "Write 3-5 bullets about") {
		t.Error("local slide plan should contain pre-filled fact instructions")
	}

	// Must reference key stats.
	if !strings.Contains(plan, "40%") {
		t.Error("local slide plan should include key stats")
	}

	// Must NOT contain meta-instructions like "→" that models copy literally.
	if strings.Contains(plan, "→") {
		t.Error("local slide plan must not contain → arrows that models copy literally")
	}
}

func TestBuildSlidePlan_Cloud(t *testing.T) {
	outline := DocOutline{
		ProjectName: "TestProject",
		Description: "A test project.",
		Sections: []OutlineSection{
			{Title: "Problem Statement", Bullets: []string{"Manual processes are slow"}},
			{Title: "Solution Overview", Bullets: []string{"Automated pipeline"}},
			{Title: "Security Audit", Bullets: []string{"Penetration tested"}},
		},
	}

	plan := buildSlidePlan(outline, "en", false)

	// Cloud plan should contain narrative structure.
	if !strings.Contains(plan, "NARRATIVE OUTLINE") {
		t.Error("cloud slide plan should contain NARRATIVE OUTLINE header")
	}

	// Should contain topic hints, not full facts.
	if !strings.Contains(plan, "Key topics:") {
		t.Error("cloud slide plan should contain topic hints")
	}

	// Should NOT contain "Facts to rephrase" (that's local-only).
	if strings.Contains(plan, "Facts to rephrase") {
		t.Error("cloud slide plan should NOT contain pre-filled facts")
	}
}

func TestBuildSlidePlan_FallbackGeneric(t *testing.T) {
	// Too few sections should trigger generic plan.
	outline := DocOutline{
		ProjectName: "Tiny",
		Description: "Small project.",
		Sections: []OutlineSection{
			{Title: "Random Heading", Bullets: []string{"something"}},
		},
	}

	plan := buildSlidePlan(outline, "en", true)

	// Generic plan should have actual Slidev markdown.
	if !strings.Contains(plan, "layout: section\n---") {
		t.Error("generic plan should contain actual Slidev section frontmatter")
	}

	// Must NOT contain → arrows.
	if strings.Contains(plan, "→") {
		t.Error("generic plan must not contain → arrows")
	}
}

func TestCleanHeading(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"**Bold Title**", "Bold Title"},
		{"LegalMind AI", "LegalMind AI"},
		{"🔒 Security", "Security"},
		{"§43a BRAO", "§43a BRAO"},        // § should be preserved
		{"2. Das Problem", "Das Problem"}, // numbered prefix stripped
		{"10. Implementierungs-Roadmap", "Implementierungs-Roadmap"},
	}

	for _, tt := range tests {
		got := cleanHeading(tt.input)
		if got != tt.want {
			t.Errorf("cleanHeading(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestShortenBullet(t *testing.T) {
	tests := []struct {
		input    string
		maxWords int
		want     string
	}{
		{"short", 5, "short"},
		{"one two three four five six seven", 5, "one two three four five"},
		{"already fine", 10, "already fine"},
	}

	for _, tt := range tests {
		got := shortenBullet(tt.input, tt.maxWords)
		if got != tt.want {
			t.Errorf("shortenBullet(%q, %d) = %q, want %q", tt.input, tt.maxWords, got, tt.want)
		}
	}
}
