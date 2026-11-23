package editorprefs

import "testing"

func TestDefaultsAndRoundTrip(t *testing.T) {
	t.Setenv("PIXELBENCH_CONFIG_DIR", t.TempDir())

	initial := Load()
	if !initial.ShowGrid || initial.OpenMode != OpenFit {
		t.Fatalf("defaults = %+v", initial)
	}

	if _, err := Save(Settings{ShowGrid: false, OpenMode: OpenActual}); err != nil {
		t.Fatal(err)
	}
	loaded := Load()
	if loaded.ShowGrid || loaded.OpenMode != OpenActual {
		t.Fatalf("loaded = %+v", loaded)
	}

	if _, err := Save(Settings{ShowGrid: true, OpenMode: "bogus"}); err != nil {
		t.Fatal(err)
	}
	if loaded := Load(); loaded.OpenMode != OpenFit {
		t.Fatalf("bad mode normalized to %q", loaded.OpenMode)
	}
}