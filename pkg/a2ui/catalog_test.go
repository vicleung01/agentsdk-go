package a2ui

import "testing"

func TestNormalizeCatalogID(t *testing.T) {
	cases := map[string]string{
		"":              BasicCatalogID,
		"basic":         BasicCatalogID,
		"basic_catalog": BasicCatalogID,
		"custom":        "custom",
		BasicCatalogID:  BasicCatalogID,
	}
	for in, want := range cases {
		if got := NormalizeCatalogID(in); got != want {
			t.Fatalf("NormalizeCatalogID(%q)=%q want %q", in, got, want)
		}
	}
}

func TestNormalize_createSurfaceCatalog(t *testing.T) {
	msg, err := ParseLine(`{"createSurface":{"surfaceId":"main","catalogId":"basic"}}`)
	if err != nil {
		t.Fatal(err)
	}
	n := msg.Normalize()
	if n.CreateSurface.CatalogID != BasicCatalogID {
		t.Fatalf("catalogId=%q", n.CreateSurface.CatalogID)
	}
}
