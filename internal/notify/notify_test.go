package notify

import "testing"

func TestAppleScriptString(t *testing.T) {
	got := AppleScriptString(`He said "hi" \ ok`)
	if got != `"He said \"hi\" \\ ok"` {
		t.Fatalf("%s", got)
	}
	if AppleScriptString("Nỏ thần\nCổ Loa") != `"Nỏ thần Cổ Loa"` {
		t.Fatal("newline not flattened")
	}
}
