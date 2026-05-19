package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestTaxonomySerializesEmptyCollectionsAsArrays(t *testing.T) {
	svc := NewService()

	body, err := json.Marshal(svc.Taxonomy(context.Background()))
	if err != nil {
		t.Fatalf("marshal taxonomy: %v", err)
	}

	for _, fragment := range []string{
		`"programs":null`,
		`"grades":null`,
		`"subjects":null`,
		`"categories":null`,
		`"sections":null`,
		`"bookSeries":null`,
		`"topics":null`,
		`"designerPresets":null`,
	} {
		if strings.Contains(string(body), fragment) {
			t.Fatalf("taxonomy should return empty arrays, got %s", body)
		}
	}
}
