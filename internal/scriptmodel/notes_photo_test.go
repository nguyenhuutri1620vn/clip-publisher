package scriptmodel

import "testing"

func TestParseNotesPhotoQueries(t *testing.T) {
	n, err := ParseNotes([]byte(`{"slides":{"s1":{"note":"cần ảnh thật","photo_queries":["Ngoc Lu bronze drum"]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	q := n.Slides["s1"].PhotoQueries
	if len(q) != 1 || q[0] != "Ngoc Lu bronze drum" {
		t.Fatalf("photo_queries = %q", q)
	}
}
