package postgres

import (
	"testing"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
)

func TestTrackOrderByUsesAllowlist(t *testing.T) {
	t.Parallel()

	for _, sort := range []metadata.TrackSort{
		metadata.TrackSortNewest,
		metadata.TrackSortOldest,
		metadata.TrackSortTitleAsc,
		metadata.TrackSortTitleDesc,
	} {
		if order, err := trackOrderBy(sort); err != nil || order == "" {
			t.Fatalf("trackOrderBy(%q) = %q, %v", sort, order, err)
		}
	}
	if _, err := trackOrderBy("title; DROP TABLE catalog.tracks"); err == nil {
		t.Fatal("trackOrderBy(unsafe value) error = nil")
	}
}

func TestEscapeLikeTreatsWildcardsAsText(t *testing.T) {
	t.Parallel()

	if result := escapeLike(`100%_mix\live`); result != `100\%\_mix\\live` {
		t.Fatalf("escapeLike() = %q", result)
	}
}
