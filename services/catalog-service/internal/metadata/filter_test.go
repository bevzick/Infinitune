package metadata

import (
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestValidateDynamicTrackQueryAcceptsSupportedFilters(t *testing.T) {
	t.Parallel()

	artistOne := uuid.New()
	artistTwo := uuid.New()
	query, err := validateDynamicTrackQuery(DynamicTrackQuery{
		Page: PageRequest{Limit: 25}, Logic: FilterLogicAnd,
		Filters: []TrackFilter{
			{Field: TrackFilterTitle, Operator: TrackFilterContains, Value: TextFilterValue("  live  ")},
			{Field: TrackFilterArtistID, Operator: TrackFilterIn, Value: UUIDListFilterValue(artistOne, artistTwo)},
			{Field: TrackFilterReleaseYear, Operator: TrackFilterBetween, Value: IntegerRangeFilterValue(2020, 2026)},
			{Field: TrackFilterDurationMS, Operator: TrackFilterGreaterThanOrEqual, Value: IntegerFilterValue(180_000)},
			{Field: TrackFilterExplicit, Operator: TrackFilterEquals, Value: BooleanFilterValue(false)},
		},
	})
	if err != nil {
		t.Fatalf("validateDynamicTrackQuery() error = %v", err)
	}
	if query.Sort != TrackSortNewest || query.Filters[0].Value.Text != "live" ||
		!reflect.DeepEqual(query.Filters[1].Value.UUIDs, []uuid.UUID{artistOne, artistTwo}) ||
		query.Filters[4].Value.Kind != TrackFilterValueBoolean || query.Filters[4].Value.Boolean {
		t.Fatalf("validateDynamicTrackQuery() = %+v", query)
	}
}

func TestValidateDynamicTrackQueryRejectsInvalidFilters(t *testing.T) {
	t.Parallel()

	validPage := PageRequest{Limit: 20}
	validFilter := TrackFilter{
		Field: TrackFilterTitle, Operator: TrackFilterContains, Value: TextFilterValue("mix"),
	}
	tests := []struct {
		name  string
		query DynamicTrackQuery
	}{
		{name: "missing logic", query: DynamicTrackQuery{Page: validPage, Filters: []TrackFilter{validFilter}}},
		{name: "empty filters", query: DynamicTrackQuery{Page: validPage, Logic: FilterLogicAnd}},
		{name: "unknown field", query: DynamicTrackQuery{Page: validPage, Logic: FilterLogicAnd, Filters: []TrackFilter{{
			Field: "title; DROP TABLE catalog.tracks", Operator: TrackFilterEquals, Value: TextFilterValue("x"),
		}}}},
		{name: "operator mismatch", query: DynamicTrackQuery{Page: validPage, Logic: FilterLogicAnd, Filters: []TrackFilter{{
			Field: TrackFilterTitle, Operator: TrackFilterGreaterThan, Value: TextFilterValue("x"),
		}}}},
		{name: "value mismatch", query: DynamicTrackQuery{Page: validPage, Logic: FilterLogicAnd, Filters: []TrackFilter{{
			Field: TrackFilterExplicit, Operator: TrackFilterEquals, Value: TextFilterValue("false"),
		}}}},
		{name: "duplicate UUIDs", query: DynamicTrackQuery{Page: validPage, Logic: FilterLogicAnd, Filters: []TrackFilter{{
			Field: TrackFilterGenreID, Operator: TrackFilterIn,
			Value: UUIDListFilterValue(uuid.MustParse("11111111-1111-1111-1111-111111111111"), uuid.MustParse("11111111-1111-1111-1111-111111111111")),
		}}}},
		{name: "reversed range", query: DynamicTrackQuery{Page: validPage, Logic: FilterLogicAnd, Filters: []TrackFilter{{
			Field: TrackFilterDurationMS, Operator: TrackFilterBetween, Value: IntegerRangeFilterValue(2000, 1000),
		}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := validateDynamicTrackQuery(test.query); !errors.Is(err, ErrInvalid) {
				t.Fatalf("validateDynamicTrackQuery() error = %v, want %v", err, ErrInvalid)
			}
		})
	}
}
