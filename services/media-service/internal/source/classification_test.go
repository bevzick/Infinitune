package source

import (
	"reflect"
	"testing"
)

func TestClassifyTagsUsesBoundedCanonicalTaxonomy(t *testing.T) {
	genres, moods := ClassifyTags([]string{"Electronica; trip hop; Chilled grooves; unknown-provider-tag"})
	if !reflect.DeepEqual(genres, []string{"electronic", "trip-hop"}) ||
		!reflect.DeepEqual(moods, []string{"relaxed"}) {
		t.Fatalf("ClassifyTags() = %v, %v", genres, moods)
	}
}
