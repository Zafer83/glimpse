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

package ai

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Zafer83/glimpse/internal/crawler"
)

// DocOutline holds the structural information extracted from project docs.
// It drives the dynamic slide plan so presentations use project-specific
// topics instead of generic placeholders like "Key Feature 1".
type DocOutline struct {
	ProjectName string
	Description string
	Sections    []OutlineSection
	KeyStats    []string
}

// OutlineSection represents a topic heading extracted from documentation.
type OutlineSection struct {
	Title   string
	Bullets []string
	Source  string // which doc file this came from
}

// narrativeSlot defines a presentation topic category with keyword matching.
type narrativeSlot struct {
	Name     string
	Keywords []string
	Labels   map[string]string // lang code → section divider title
	ImageKW  string            // default image keywords for this topic
	Layout   string            // preferred layout for first content slide
}

// defaultNarrativeSlots defines the canonical presentation narrative order.
// Each slot can be filled by matching doc sections via keyword matching.
var defaultNarrativeSlots = []narrativeSlot{
	{
		Name:     "problem",
		Keywords: []string{"problem", "herausforderung", "challenge", "motivation", "ausgangslage", "status quo", "warum", "why"},
		Labels:   map[string]string{"de": "Das Problem", "en": "The Problem", "fr": "Le Problème", "es": "El Problema"},
		Layout:   "default",
	},
	{
		Name:     "solution",
		Keywords: []string{"lösung", "solution", "ansatz", "approach", "überblick", "overview", "agentic", "konzept"},
		Labels:   map[string]string{"de": "Die Lösung", "en": "The Solution", "fr": "La Solution", "es": "La Solución"},
		ImageKW:  "technology,innovation",
		Layout:   "image-right",
	},
	{
		Name:     "features",
		Keywords: []string{"feature", "use case", "anwendung", "funktion", "modul", "agent", "fähigkeit", "capability", "workflow", "tool"},
		Labels:   map[string]string{"de": "Features & Use Cases", "en": "Features & Use Cases", "fr": "Fonctionnalités", "es": "Funcionalidades"},
		ImageKW:  "data,analysis",
		Layout:   "default",
	},
	{
		Name:     "architecture",
		Keywords: []string{"architektur", "architecture", "system", "framework", "infrastruktur", "stack", "technolog", "pipeline", "rag", "hardware"},
		Labels:   map[string]string{"de": "Technische Architektur", "en": "Technical Architecture", "fr": "Architecture Technique", "es": "Arquitectura Técnica"},
		ImageKW:  "server,network",
		Layout:   "image-right",
	},
	{
		Name:     "security",
		Keywords: []string{"sicherheit", "security", "compliance", "dsgvo", "datenschutz", "gdpr", "recht", "brao", "audit", "governance", "guardrail", "safety"},
		Labels:   map[string]string{"de": "Compliance & Sicherheit", "en": "Compliance & Security", "fr": "Conformité & Sécurité", "es": "Cumplimiento & Seguridad"},
		ImageKW:  "security,lock",
		Layout:   "image-left",
	},
	{
		Name:     "business",
		Keywords: []string{"geschäft", "business", "pricing", "preis", "roi", "wirtschaft", "markt", "wettbewerb", "competition", "deploy", "saas"},
		Labels:   map[string]string{"de": "Geschäftsmodell", "en": "Business Model", "fr": "Modèle Commercial", "es": "Modelo de Negocio"},
		ImageKW:  "business,chart",
		Layout:   "default",
	},
	{
		Name:     "roadmap",
		Keywords: []string{"roadmap", "timeline", "plan", "phase", "zukunft", "future", "milestone", "todo", "nächste", "next", "ausblick", "evaluation"},
		Labels:   map[string]string{"de": "Roadmap & Ausblick", "en": "Roadmap & Outlook", "fr": "Feuille de Route", "es": "Hoja de Ruta"},
		Layout:   "default",
	},
}

// mdLinkRe matches [text](url) markdown links for stripping.
var mdLinkRe = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)

// extractDocOutline parses project documentation and extracts a structured
// outline: project name, description, topic sections with bullets, and key
// statistics. The outline drives the dynamic slide plan builder.
func extractDocOutline(docs []crawler.FileEntry) DocOutline {
	var outline DocOutline
	seenTitles := make(map[string]bool)

	for _, doc := range docs {
		lines := strings.Split(doc.Content, "\n")
		var currentSection *OutlineSection
		descFound := outline.Description != ""

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)

			// Project name from first H1 heading.
			if outline.ProjectName == "" && strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
				name := cleanHeading(strings.TrimPrefix(trimmed, "# "))
				if name != "" {
					outline.ProjectName = name
				}
				continue
			}

			// Description: first substantial non-heading, non-link line.
			if outline.ProjectName != "" && !descFound && trimmed != "" &&
				!strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "[") &&
				!strings.HasPrefix(trimmed, "!") && !strings.HasPrefix(trimmed, "|") &&
				!strings.HasPrefix(trimmed, "```") && !strings.HasPrefix(trimmed, "---") &&
				len(trimmed) > 20 {
				// Strip markdown formatting for cleaner description.
				desc := mdLinkRe.ReplaceAllString(trimmed, "$1")
				desc = strings.ReplaceAll(desc, "**", "")
				outline.Description = desc
				descFound = true
				continue
			}

			// H2 = new topic section.
			if strings.HasPrefix(trimmed, "## ") {
				title := cleanHeading(strings.TrimPrefix(trimmed, "## "))
				key := strings.ToLower(title)
				// Skip table-of-contents headings — they add no content.
				if key == "inhaltsverzeichnis" || key == "table of contents" || key == "toc" || key == "contents" {
					currentSection = nil
					continue
				}
				if title != "" && !seenTitles[key] && len(title) < 100 {
					seenTitles[key] = true
					outline.Sections = append(outline.Sections, OutlineSection{
						Title:  title,
						Source: doc.Path,
					})
					currentSection = &outline.Sections[len(outline.Sections)-1]
				} else {
					currentSection = nil
				}
				continue
			}

			// Collect bullets under current section (max 8 per section).
			if currentSection != nil && len(currentSection.Bullets) < 8 {
				isBullet := strings.HasPrefix(trimmed, "- ") ||
					strings.HasPrefix(trimmed, "* ") ||
					strings.HasPrefix(trimmed, "+ ")
				isNumbered := len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' &&
					(trimmed[1] == '.' || (len(trimmed) > 2 && trimmed[2] == '.'))

				if isBullet || isNumbered {
					bullet := trimmed
					if isBullet {
						bullet = trimmed[2:]
					} else if isNumbered {
						dotIdx := strings.Index(bullet, ".")
						if dotIdx > 0 && dotIdx < 3 {
							bullet = strings.TrimSpace(bullet[dotIdx+1:])
						}
					}
					// Strip markdown links and bold.
					bullet = mdLinkRe.ReplaceAllString(bullet, "$1")
					bullet = strings.ReplaceAll(bullet, "**", "")
					bullet = strings.TrimSpace(bullet)
					if len(bullet) > 5 && len(bullet) < 150 {
						currentSection.Bullets = append(currentSection.Bullets, bullet)
					}
				}
			}
		}
	}

	outline.KeyStats = extractKeyStats(docs)
	return outline
}

// numberPrefixRe matches leading "1. ", "12. " etc. in headings.
var numberPrefixRe = regexp.MustCompile(`^\d{1,3}\.\s+`)

// cleanHeading strips markdown formatting, leading emoji, and numbered prefixes.
func cleanHeading(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "*", "")
	// Strip leading non-ASCII characters (emoji, symbols) but keep § and digits.
	for len(s) > 0 {
		r := s[0]
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '(' {
			break
		}
		// Keep § character (UTF-8: C2 A7).
		if r == 0xC2 && len(s) > 1 && s[1] == 0xA7 {
			break
		}
		if r > 127 {
			// Skip multi-byte UTF-8 character.
			i := 1
			for i < len(s) && (s[i]&0xC0) == 0x80 {
				i++
			}
			s = strings.TrimSpace(s[i:])
		} else {
			s = strings.TrimSpace(s[1:])
		}
	}
	// Strip numbered prefix ("1. ", "12. ") for cleaner slide titles.
	s = numberPrefixRe.ReplaceAllString(s, "")
	return s
}

// keyStatRe matches numbers with units, percentages, and legal references.
var keyStatRe = regexp.MustCompile(
	`(?i)(?:` +
		`\d+[\.,]?\d*\s*%|` + // 40%, 65.5%
		`\d+[\.,]?\d*\s*(?:EUR|USD|€|\$)|` + // 159 EUR
		`\d+[–-]\d+\s*%|` + // 40-60%
		`\d+[–-]\d+[\.,]?\d*\s*(?:EUR|USD|€)|` + // 1.000-1.200 EUR
		`\d+[–-]\d+\s*(?:Stunden|hours?)|` + // 2-4 Stunden
		`§\s*\d+\w?\s+\w+` + // §43a BRAO
		`)`,
)

// extractKeyStats finds quantitative data in docs: percentages, prices, legal refs.
func extractKeyStats(docs []crawler.FileEntry) []string {
	seen := make(map[string]bool)
	var stats []string
	for _, doc := range docs {
		for _, m := range keyStatRe.FindAllString(doc.Content, -1) {
			m = strings.TrimSpace(m)
			if !seen[m] && len(m) > 2 {
				seen[m] = true
				stats = append(stats, m)
				if len(stats) >= 20 {
					return stats
				}
			}
		}
	}
	return stats
}

// mapSectionsToSlots assigns extracted doc sections to narrative slots via
// keyword matching on section titles. Unmatched sections overflow into "features".
func mapSectionsToSlots(sections []OutlineSection) map[string][]OutlineSection {
	result := make(map[string][]OutlineSection)
	used := make(map[int]bool)

	for _, slot := range defaultNarrativeSlots {
		for i, sec := range sections {
			if used[i] {
				continue
			}
			lower := strings.ToLower(sec.Title)
			for _, kw := range slot.Keywords {
				if strings.Contains(lower, kw) {
					result[slot.Name] = append(result[slot.Name], sec)
					used[i] = true
					break
				}
			}
		}
		// Keep max 3 sections per slot.
		if len(result[slot.Name]) > 3 {
			result[slot.Name] = result[slot.Name][:3]
		}
	}

	// Unmatched sections go into "features" as additional content.
	for i, sec := range sections {
		if !used[i] {
			result["features"] = append(result["features"], sec)
		}
	}

	return result
}

// slotLabel returns the localized section divider title for a slot.
func slotLabel(slot narrativeSlot, lang string) string {
	lang = strings.ToLower(lang)
	if l, ok := slot.Labels[lang]; ok {
		return l
	}
	return slot.Labels["en"]
}

// shortenBullet truncates a bullet to maxWords words.
func shortenBullet(bullet string, maxWords int) string {
	words := strings.Fields(bullet)
	if len(words) <= maxWords {
		return bullet
	}
	return strings.Join(words[:maxWords], " ")
}

// buildSlidePlan generates a project-specific slide plan from the document
// outline. For local models (forLocal=true), the plan includes pre-extracted
// bullet facts that the model should rephrase into short slide bullets.
// For cloud models, it provides narrative structure with topic hints and lets
// the model create rich content with HTML, tables, and data visualizations.
func buildSlidePlan(outline DocOutline, lang string, forLocal bool) string {
	slots := mapSectionsToSlots(outline.Sections)

	// Count how many narrative slots have matching content.
	filled := 0
	for _, slot := range defaultNarrativeSlots {
		if len(slots[slot.Name]) > 0 {
			filled++
		}
	}
	if filled < 2 {
		return buildGenericSlidePlan(outline, lang, forLocal)
	}

	var b strings.Builder

	if forLocal {
		return buildLocalSlidePlan(outline, slots, lang)
	}

	// Cloud model: narrative outline with topic hints.
	b.WriteString("NARRATIVE OUTLINE — follow this structure to create 15-20 slides.\n")
	b.WriteString("Add section divider slides (layout: section) between major topics.\n")
	b.WriteString("Use specific numbers and data from the documentation.\n")
	b.WriteString("You may use markdown tables for comparisons and data.\n\n")

	slideNum := 2
	for _, slot := range defaultNarrativeSlots {
		sections, ok := slots[slot.Name]
		if !ok || len(sections) == 0 {
			continue
		}

		label := slotLabel(slot, lang)
		b.WriteString(fmt.Sprintf("[Slide %d] Section divider: # %s\n", slideNum, label))
		slideNum++

		maxContent := 2
		if slot.Name == "features" || slot.Name == "architecture" {
			maxContent = 3
		}

		for ci, sec := range sections {
			if ci >= maxContent {
				break
			}

			layout := "default"
			imgKW := ""
			if ci == 0 && (slot.Layout == "image-right" || slot.Layout == "image-left") {
				layout = slot.Layout
				imgKW = slot.ImageKW
			}

			b.WriteString(fmt.Sprintf("[Slide %d] layout: %s — # %s", slideNum, layout, sec.Title))
			if imgKW != "" {
				b.WriteString(fmt.Sprintf(" (image: %s)", imgKW))
			}
			b.WriteString("\n")

			if len(sec.Bullets) > 0 {
				b.WriteString("  Key topics: ")
				limit := min(5, len(sec.Bullets))
				topics := make([]string, 0, limit)
				for _, bullet := range sec.Bullets[:limit] {
					topics = append(topics, shortenBullet(bullet, 8))
				}
				b.WriteString(strings.Join(topics, "; ") + "\n")
			}
			slideNum++
		}
		b.WriteString("\n")
	}

	if len(outline.KeyStats) > 0 {
		b.WriteString("KEY STATISTICS found in documentation (use these in your slides):\n")
		limit := min(15, len(outline.KeyStats))
		for _, stat := range outline.KeyStats[:limit] {
			b.WriteString(fmt.Sprintf("  • %s\n", stat))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// buildLocalSlidePlan generates a concrete, copy-paste-ready slide skeleton
// for local models. Small 7B models cannot interpret meta-instructions like
// "create a section divider slide"; they need the exact Slidev markdown so
// they only have to fill in bullet content between the provided headings.
func buildLocalSlidePlan(outline DocOutline, slots map[string][]OutlineSection, lang string) string {
	var b strings.Builder

	b.WriteString("SLIDE PLAN — output these slides EXACTLY as shown.\n")
	b.WriteString("For each content slide: add 3-5 SHORT bullets (max 8 words each).\n")
	b.WriteString("For DIVIDER slides: output them EXACTLY, nothing else.\n\n")

	for _, slot := range defaultNarrativeSlots {
		sections, ok := slots[slot.Name]
		if !ok || len(sections) == 0 {
			continue
		}

		label := slotLabel(slot, lang)

		// Write the section divider as exact Slidev markdown.
		b.WriteString("---\nlayout: section\n---\n\n")
		b.WriteString(fmt.Sprintf("# %s\n\n", label))

		maxContent := 2
		if slot.Name == "features" || slot.Name == "architecture" {
			maxContent = 3
		}

		for ci, sec := range sections {
			if ci >= maxContent {
				break
			}

			// Write content slide frontmatter.
			if ci == 0 && slot.Layout == "image-right" && slot.ImageKW != "" {
				b.WriteString("---\nlayout: image-right\n")
				b.WriteString(fmt.Sprintf("image: %s\n", slot.ImageKW))
				b.WriteString("---\n\n")
			} else if ci == 0 && slot.Layout == "image-left" && slot.ImageKW != "" {
				b.WriteString("---\nlayout: image-left\n")
				b.WriteString(fmt.Sprintf("image: %s\n", slot.ImageKW))
				b.WriteString("---\n\n")
			} else {
				b.WriteString("---\n\n")
			}

			b.WriteString(fmt.Sprintf("# %s\n\n", sec.Title))

			// Pre-fill facts the model should rephrase.
			if len(sec.Bullets) > 0 {
				b.WriteString("Write 3-5 bullets about:\n")
				limit := min(5, len(sec.Bullets))
				for _, bullet := range sec.Bullets[:limit] {
					b.WriteString(fmt.Sprintf("- %s\n", shortenBullet(bullet, 12)))
				}
			} else {
				b.WriteString("Write 3-4 bullets about this topic.\n")
			}
			b.WriteString("\n")
		}
	}

	if len(outline.KeyStats) > 0 {
		b.WriteString("KEY STATISTICS — use these numbers in your bullets:\n")
		limit := min(12, len(outline.KeyStats))
		for _, stat := range outline.KeyStats[:limit] {
			b.WriteString(fmt.Sprintf("  %s\n", stat))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// buildGenericSlidePlan creates a fallback slide plan when the documentation
// has too few H2 headings for a dynamic plan.
func buildGenericSlidePlan(outline DocOutline, lang string, forLocal bool) string {
	var b strings.Builder

	if forLocal {
		b.WriteString("SLIDE PLAN — output these slides EXACTLY as shown.\n")
		b.WriteString("For each content slide: add 3-5 SHORT bullets (max 8 words each).\n\n")

		b.WriteString("---\nlayout: section\n---\n\n# The Problem\n\n")
		b.WriteString("---\n\n# What Problem Does This Solve?\n\nWrite 3-4 bullets.\n\n")

		b.WriteString("---\nlayout: section\n---\n\n# The Solution\n\n")
		b.WriteString("---\nlayout: image-right\nimage: technology,solution\n---\n\n# How It Works\n\nWrite 3-4 bullets.\n\n")

		b.WriteString("---\nlayout: section\n---\n\n# Architecture\n\n")
		b.WriteString("---\n\n# System Components\n\nWrite 3-5 bullets about the architecture.\n\n")

		b.WriteString("---\nlayout: section\n---\n\n# Features\n\n")
		b.WriteString("---\n\n# Key Features\n\nWrite 3-4 bullets about the most important features.\n\n")
		b.WriteString("---\nlayout: image-left\nimage: data,analysis\n---\n\n# More Features\n\nWrite 3-4 bullets.\n\n")

		b.WriteString("---\nlayout: two-cols\n---\n\n# Tech Stack\n\nBackend technologies\n\n::right::\n\nFrontend/Infra technologies\n\n")
	} else {
		b.WriteString("NARRATIVE OUTLINE — create 15-20 slides with these topics:\n\n")
		b.WriteString("1. Problem & Motivation (with section divider)\n")
		b.WriteString("2. Solution Overview (with section divider)\n")
		b.WriteString("3. Architecture & Tech Stack (with section divider)\n")
		b.WriteString("4. Key Features — 2-3 content slides (with section divider)\n")
		b.WriteString("5. Code Examples — 1-2 slides\n")
		b.WriteString("6. Deployment & Usage (with section divider)\n\n")
		b.WriteString("Add section divider slides (layout: section) between each major topic.\n")
	}

	if len(outline.KeyStats) > 0 {
		b.WriteString("\nKEY STATISTICS found in documentation:\n")
		for _, s := range outline.KeyStats {
			b.WriteString(fmt.Sprintf("  • %s\n", s))
		}
		b.WriteString("\n")
	}

	return b.String()
}
