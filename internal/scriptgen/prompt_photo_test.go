package scriptgen

import (
	"strings"
	"testing"
)

func TestPromptAsksForPhotoQueries(t *testing.T) {
	p := BuildPrompt("", nil, Request{Input: "x"})
	if !strings.Contains(p, "photo_queries") {
		t.Fatal("prompt does not ask for notes.slides.<id>.photo_queries")
	}
}
