package music

import "testing"

func TestExtractVideoID(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ":          "dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ":                         "dQw4w9WgXcQ",
		"https://www.youtube.com/embed/dQw4w9WgXcQ":            "dQw4w9WgXcQ",
		"https://youtube.com/shorts/dQw4w9WgXcQ":               "dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=xyz": "dQw4w9WgXcQ",
	}
	for in, want := range cases {
		got, ok := ExtractVideoID(in)
		if !ok || got != want {
			t.Errorf("ExtractVideoID(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
}

func TestExtractVideoIDInvalid(t *testing.T) {
	for _, in := range []string{"https://example.com/x", "not a url", "https://vimeo.com/123"} {
		if _, ok := ExtractVideoID(in); ok {
			t.Errorf("expected %q to be rejected", in)
		}
	}
}
