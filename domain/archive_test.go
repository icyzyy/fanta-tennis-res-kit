package domain

import "testing"

func TestKindOf(t *testing.T) {
	cases := map[string]EntryKind{
		"Gui_Login.set":      KindSet,
		"Script/a/b.SET":     KindSet,
		"UI01_Title.tex":     KindTex,
		"dir/NAME.Tex":       KindTex,
		"readme.txt":         KindOther,
		"noext":              KindOther,
		"archive.set.backup": KindOther,
		"":                   KindOther,
	}
	for name, want := range cases {
		if got := KindOf(name); got != want {
			t.Errorf("KindOf(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestEntryKindString(t *testing.T) {
	if KindSet.String() != "set" || KindTex.String() != "tex" || KindOther.String() != "other" {
		t.Errorf("String() wrong: %q %q %q", KindSet, KindTex, KindOther)
	}
}
