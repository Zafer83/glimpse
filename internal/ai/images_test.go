package ai

import (
	"strings"
	"testing"
)

func TestMatchImage_ExactKeyword(t *testing.T) {
	photoID := matchImage("code", contentImages)
	// Should match the code/programming entry.
	if photoID == "" {
		t.Error("matchImage returned empty photo ID")
	}
	// Verify it's a real entry.
	found := false
	for _, e := range contentImages {
		if e.PhotoID == photoID {
			found = true
		}
	}
	if !found {
		t.Errorf("matchImage returned unknown photo ID: %s", photoID)
	}
}

func TestMatchImage_MultipleKeywords(t *testing.T) {
	photoID := matchImage("law,contract", contentImages)
	// Should match the legal/contract entry.
	if photoID == "" {
		t.Error("matchImage returned empty for law,contract")
	}
}

func TestMatchImage_Fallback(t *testing.T) {
	// Nonsense keywords should still return the first entry as fallback.
	photoID := matchImage("xyznonexistent", contentImages)
	if photoID != contentImages[0].PhotoID {
		t.Errorf("expected fallback to first entry, got %s", photoID)
	}
}

func TestResolveImageToURL_Cover(t *testing.T) {
	url := resolveImageToURL("code,technology", true)
	if !strings.Contains(url, "images.unsplash.com/photo-") {
		t.Errorf("cover URL should use images.unsplash.com, got: %s", url)
	}
	if !strings.Contains(url, "w=1600") || !strings.Contains(url, "h=900") {
		t.Errorf("cover URL should use 1600x900 crop, got: %s", url)
	}
}

func TestResolveImageToURL_Content(t *testing.T) {
	url := resolveImageToURL("data,analysis", false)
	if !strings.Contains(url, "images.unsplash.com/photo-") {
		t.Errorf("content URL should use images.unsplash.com, got: %s", url)
	}
	if !strings.Contains(url, "w=800") || !strings.Contains(url, "h=1000") {
		t.Errorf("content URL should use 800x1000 crop, got: %s", url)
	}
}

func TestResolveAllImageKeywords_ReplacesKeywords(t *testing.T) {
	body := "---\nlayout: image-right\nimage: law,contract\n---\n\n# Legal\n\nSome content."
	result := resolveAllImageKeywords(body)

	if strings.Contains(result, "image: law,contract") {
		t.Error("keywords should have been replaced with URL")
	}
	if !strings.Contains(result, "images.unsplash.com/photo-") {
		t.Error("result should contain an Unsplash CDN URL")
	}
	// image-right should get portrait crop.
	if !strings.Contains(result, "w=800") {
		t.Error("image-right should use portrait crop (w=800)")
	}
}

func TestResolveAllImageKeywords_PreservesExistingURLs(t *testing.T) {
	body := "---\nlayout: image-right\nimage: https://images.unsplash.com/photo-existing\n---\n\n# Slide"
	result := resolveAllImageKeywords(body)

	if !strings.Contains(result, "photo-existing") {
		t.Error("existing URLs should be preserved")
	}
}

func TestResolveAllImageKeywords_CoverBackground(t *testing.T) {
	body := "---\nlayout: cover\nbackground: technology,dark\n---\n\n# Title"
	result := resolveAllImageKeywords(body)

	if strings.Contains(result, "background: technology,dark") {
		t.Error("background keywords should have been replaced")
	}
	// Cover backgrounds should get landscape crop.
	if !strings.Contains(result, "w=1600") {
		t.Error("cover background should use landscape crop (w=1600)")
	}
}

func TestResolveAllImageKeywords_SectionLayout(t *testing.T) {
	body := "---\nlayout: section\nbackground: abstract,dark\n---\n\n# Section Title"
	result := resolveAllImageKeywords(body)

	// Section layouts should also get cover crop.
	if strings.Contains(result, "abstract,dark") && !strings.Contains(result, "images.unsplash.com") {
		t.Error("section background keywords should be resolved")
	}
}

func TestResolveAllImageKeywords_NoFrontmatter(t *testing.T) {
	// Content outside frontmatter should not be touched.
	body := "# Slide\n\nimage: some,keywords\n\n---\n\n# Next"
	result := resolveAllImageKeywords(body)
	if !strings.Contains(result, "image: some,keywords") {
		t.Error("image values outside frontmatter should not be modified")
	}
}
