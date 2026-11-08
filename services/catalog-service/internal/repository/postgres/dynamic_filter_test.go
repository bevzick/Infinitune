package postgres

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/google/uuid"
)

func TestDynamicTrackPredicateParameterizesValues(t *testing.T) {
	t.Parallel()

	maliciousTitle := `%' OR TRUE; DROP TABLE catalog.tracks; --`
	predicate, arguments, err := dynamicTrackPredicate(metadata.DynamicTrackQuery{
		Logic: metadata.FilterLogicOr,
		Filters: []metadata.TrackFilter{
			{Field: metadata.TrackFilterTitle, Operator: metadata.TrackFilterContains, Value: metadata.TextFilterValue(maliciousTitle)},
			{Field: metadata.TrackFilterDurationMS, Operator: metadata.TrackFilterBetween, Value: metadata.IntegerRangeFilterValue(120_000, 300_000)},
		},
	})
	if err != nil {
		t.Fatalf("dynamicTrackPredicate() error = %v", err)
	}
	if strings.Contains(predicate, maliciousTitle) || !strings.Contains(predicate, "$1") ||
		!strings.Contains(predicate, "$2") || !strings.Contains(predicate, "$3") ||
		!strings.Contains(predicate, " OR ") {
		t.Fatalf("dynamicTrackPredicate() predicate = %q", predicate)
	}
	wantArguments := []any{escapeLike(maliciousTitle), 120_000, 300_000}
	if !reflect.DeepEqual(arguments, wantArguments) {
		t.Fatalf("dynamicTrackPredicate() arguments = %#v, want %#v", arguments, wantArguments)
	}
}

func TestTrackFilterPredicateUsesAllowlistedFragments(t *testing.T) {
	t.Parallel()

	idOne := uuid.New()
	idTwo := uuid.New()
	filters := []metadata.TrackFilter{
		{Field: metadata.TrackFilterTitle, Operator: metadata.TrackFilterEquals, Value: metadata.TextFilterValue("Exact")},
		{Field: metadata.TrackFilterArtistID, Operator: metadata.TrackFilterIn, Value: metadata.UUIDListFilterValue(idOne, idTwo)},
		{Field: metadata.TrackFilterAlbumID, Operator: metadata.TrackFilterEquals, Value: metadata.UUIDFilterValue(idOne)},
		{Field: metadata.TrackFilterGenreID, Operator: metadata.TrackFilterEquals, Value: metadata.UUIDFilterValue(idOne)},
		{Field: metadata.TrackFilterMoodID, Operator: metadata.TrackFilterIn, Value: metadata.UUIDListFilterValue(idOne)},
		{Field: metadata.TrackFilterReleaseYear, Operator: metadata.TrackFilterLessThanOrEqual, Value: metadata.IntegerFilterValue(2026)},
		{Field: metadata.TrackFilterDurationMS, Operator: metadata.TrackFilterGreaterThan, Value: metadata.IntegerFilterValue(1)},
		{Field: metadata.TrackFilterExplicit, Operator: metadata.TrackFilterEquals, Value: metadata.BooleanFilterValue(false)},
	}
	for _, filter := range filters {
		arguments := make([]any, 0)
		if predicate, err := trackFilterPredicate(filter, &arguments); err != nil || predicate == "" || len(arguments) == 0 {
			t.Fatalf("trackFilterPredicate(%+v) = %q, %#v, %v", filter, predicate, arguments, err)
		}
	}

	arguments := make([]any, 0)
	_, err := trackFilterPredicate(metadata.TrackFilter{
		Field: "title) OR TRUE --", Operator: metadata.TrackFilterEquals,
		Value: metadata.TextFilterValue("unsafe"),
	}, &arguments)
	if !errors.Is(err, metadata.ErrInvalid) || len(arguments) != 0 {
		t.Fatalf("unsafe filter error = %v, arguments = %#v", err, arguments)
	}
}
