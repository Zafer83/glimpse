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
	"strings"
)

// imageEntry holds an Unsplash photo ID with metadata for matching.
type imageEntry struct {
	PhotoID  string   // Unsplash photo ID (e.g. "1555066931-4365d14bab8c")
	Keywords []string // keywords this image matches
}

// coverImages are landscape photos with lots of negative space (dark backgrounds,
// minimalist, abstract textures) — ideal for cover/section slides where text
// must remain readable.
var coverImages = []imageEntry{
	// Dark abstract code/tech
	{PhotoID: "1555066931-4365d14bab8c", Keywords: []string{"code", "programming", "software", "developer"}},
	// Dark gradient abstract
	{PhotoID: "1557683316-973673baf926", Keywords: []string{"abstract", "gradient", "dark", "minimal"}},
	// Dark blue network/tech
	{PhotoID: "1451187580459-43490279c0fa", Keywords: []string{"network", "technology", "data", "global"}},
	// Dark workspace minimal
	{PhotoID: "1516116216624-53e697fedbea", Keywords: []string{"workspace", "desk", "office", "work"}},
	// Abstract dark lines
	{PhotoID: "1558618666-fcd25c85f7f7", Keywords: []string{"lines", "pattern", "geometric", "design"}},
	// Server room / infrastructure
	{PhotoID: "1558494949-ef010cbdcc31", Keywords: []string{"server", "infrastructure", "cloud", "datacenter"}},
	// Dark mountain landscape
	{PhotoID: "1519681393784-d120267933ba", Keywords: []string{"nature", "mountain", "landscape", "dark"}},
	// Circuit board / hardware
	{PhotoID: "1518770660439-4636190af475", Keywords: []string{"hardware", "circuit", "electronics", "chip"}},
	// Abstract blue particles
	{PhotoID: "1635070041078-e363dbe005cb", Keywords: []string{"particles", "ai", "intelligence", "neural"}},
	// Legal / library / books
	{PhotoID: "1589829545856-d10d557cf95f", Keywords: []string{"law", "legal", "justice", "library"}},
	// Finance / chart
	{PhotoID: "1611974789855-9c2a0a7236a3", Keywords: []string{"finance", "chart", "business", "analytics"}},
	// Medical / health
	{PhotoID: "1576091160399-112ba8d25d1d", Keywords: []string{"medical", "health", "science", "research"}},
	// Security / lock
	{PhotoID: "1555949963-aa79dcee981c", Keywords: []string{"security", "lock", "protection", "privacy"}},
	// Dark ocean / minimal
	{PhotoID: "1505118380757-91f5816fff7e", Keywords: []string{"ocean", "water", "calm", "minimal"}},
	// Cityscape night
	{PhotoID: "1477959858617-67f85cf4f1df", Keywords: []string{"city", "night", "urban", "skyline"}},
}

// contentImages are photos suitable for image-right / image-left layouts.
// They work well when cropped to portrait or square aspect ratios in the
// narrower column of a two-column Slidev layout.
var contentImages = []imageEntry{
	// Code on screen (vertical crop works)
	{PhotoID: "1461749280684-dccba630e2f6", Keywords: []string{"code", "programming", "developer", "software"}},
	// Server rack detail
	{PhotoID: "1558494949-ef010cbdcc31", Keywords: []string{"server", "cloud", "infrastructure", "deploy"}},
	// Abstract network nodes
	{PhotoID: "1558618666-fcd25c85f7f7", Keywords: []string{"network", "architecture", "system", "connection"}},
	// Data visualization
	{PhotoID: "1551288049-bebda4e38f71", Keywords: []string{"data", "analysis", "chart", "dashboard"}},
	// Team collaboration
	{PhotoID: "1522071820081-009f0129c71c", Keywords: []string{"team", "collaboration", "people", "meeting"}},
	// Shield / security
	{PhotoID: "1555949963-aa79dcee981c", Keywords: []string{"security", "lock", "shield", "protection"}},
	// Gears / automation
	{PhotoID: "1537151608828-ea2b11777ee8", Keywords: []string{"automation", "process", "workflow", "gear"}},
	// Technology / innovation
	{PhotoID: "1535378917042-10a22c95931a", Keywords: []string{"technology", "innovation", "future", "smart"}},
	// Legal / contract / document
	{PhotoID: "1450101499163-c8848e968838", Keywords: []string{"law", "legal", "contract", "document", "justice"}},
	// Blueprint / architecture
	{PhotoID: "1503387762-592deb58ef4e", Keywords: []string{"architecture", "blueprint", "plan", "building"}},
	// Rocket / launch / startup
	{PhotoID: "1517976487171-060c467d77c8", Keywords: []string{"launch", "startup", "rocket", "growth"}},
	// Money / pricing / business
	{PhotoID: "1611974789855-9c2a0a7236a3", Keywords: []string{"business", "pricing", "money", "finance"}},
	// AI / neural / brain
	{PhotoID: "1635070041078-e363dbe005cb", Keywords: []string{"ai", "neural", "intelligence", "brain"}},
	// Compliance / checklist
	{PhotoID: "1484480974693-6ca0a78fb36b", Keywords: []string{"compliance", "checklist", "audit", "quality"}},
	// Road / roadmap / path
	{PhotoID: "1470071459604-3b5ec3a7fe05", Keywords: []string{"roadmap", "path", "road", "future", "outlook"}},
}

// unsplashCoverURL builds an Unsplash CDN URL optimized for cover/section slides.
// Uses landscape crop (1600x900) for full-width backgrounds.
func unsplashCoverURL(photoID string) string {
	return fmt.Sprintf("https://images.unsplash.com/photo-%s?auto=format&fit=crop&w=1600&h=900&q=80", photoID)
}

// unsplashContentURL builds an Unsplash CDN URL optimized for image-right/left.
// Uses portrait-ish crop (800x1000) for the narrow column in two-column layouts.
func unsplashContentURL(photoID string) string {
	return fmt.Sprintf("https://images.unsplash.com/photo-%s?auto=format&fit=crop&w=800&h=1000&q=80", photoID)
}

// matchImage finds the best matching photo for the given keywords from a pool.
// Returns the photo ID or falls back to the first entry if nothing matches.
func matchImage(keywords string, pool []imageEntry) string {
	kws := strings.Split(strings.ToLower(keywords), ",")
	for i := range kws {
		kws[i] = strings.TrimSpace(kws[i])
	}

	bestID := pool[0].PhotoID
	bestScore := 0

	for _, entry := range pool {
		score := 0
		for _, userKW := range kws {
			if userKW == "" {
				continue
			}
			for _, entryKW := range entry.Keywords {
				if userKW == entryKW || strings.Contains(entryKW, userKW) || strings.Contains(userKW, entryKW) {
					score++
				}
			}
		}
		if score > bestScore {
			bestScore = score
			bestID = entry.PhotoID
		}
	}

	return bestID
}

// resolveImageToURL converts keyword-style image values to proper Unsplash CDN
// URLs using curated photo IDs. The layout context determines the crop:
//   - cover/section backgrounds: landscape 1600x900
//   - image-right/image-left: portrait 800x1000
func resolveImageToURL(keywords string, isCoverLayout bool) string {
	if isCoverLayout {
		photoID := matchImage(keywords, coverImages)
		return unsplashCoverURL(photoID)
	}
	photoID := matchImage(keywords, contentImages)
	return unsplashContentURL(photoID)
}

// resolveAllImageKeywords walks the Slidev body and replaces keyword-style
// image: and background: values with proper Unsplash CDN URLs.
// It detects the layout context (cover/section vs image-right/left) to pick
// the appropriate image crop dimensions.
func resolveAllImageKeywords(body string) string {
	lines := strings.Split(body, "\n")
	inFrontmatter := false
	currentLayout := ""

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "---" {
			if inFrontmatter {
				// Closing --- : reset for next slide.
				inFrontmatter = false
				currentLayout = ""
			} else {
				inFrontmatter = true
				currentLayout = ""
			}
			continue
		}

		if !inFrontmatter {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(parts[0]))

		// Track layout for crop decision.
		if key == "layout" {
			currentLayout = strings.TrimSpace(strings.Trim(parts[1], `"' `))
			continue
		}

		// Process image: and background: values.
		if key != "image" && key != "background" {
			continue
		}

		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`)

		// Already a URL — leave untouched.
		if strings.HasPrefix(val, "http") {
			continue
		}
		if val == "" {
			continue
		}

		// Determine crop based on layout context.
		isCover := key == "background" ||
			currentLayout == "cover" ||
			currentLayout == "section"

		resolved := resolveImageToURL(val, isCover)
		lines[i] = parts[0] + ": " + resolved
	}

	return strings.Join(lines, "\n")
}
