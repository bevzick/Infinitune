package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/google/uuid"
)

func (repository *MetadataRepository) EvaluateTrackFilters(
	ctx context.Context,
	query metadata.DynamicTrackQuery,
) (metadata.Page[metadata.Track], error) {
	orderBy, err := trackOrderBy(query.Sort)
	if err != nil {
		return metadata.Page[metadata.Track]{}, err
	}
	predicate, arguments, err := dynamicTrackPredicate(query)
	if err != nil {
		return metadata.Page[metadata.Track]{}, err
	}
	return repository.filteredTrackPage(
		ctx,
		" FROM catalog.tracks AS t WHERE "+predicate,
		arguments,
		query.Page,
		orderBy,
	)
}

func dynamicTrackPredicate(query metadata.DynamicTrackQuery) (string, []any, error) {
	if query.Logic != metadata.FilterLogicAnd && query.Logic != metadata.FilterLogicOr {
		return "", nil, fmt.Errorf("unsupported filter logic %q: %w", query.Logic, metadata.ErrInvalid)
	}
	if len(query.Filters) == 0 {
		return "", nil, fmt.Errorf("empty track filters: %w", metadata.ErrInvalid)
	}

	predicates := make([]string, 0, len(query.Filters))
	arguments := make([]any, 0, len(query.Filters))
	for _, filter := range query.Filters {
		predicate, err := trackFilterPredicate(filter, &arguments)
		if err != nil {
			return "", nil, err
		}
		predicates = append(predicates, "("+predicate+")")
	}
	return strings.Join(predicates, " "+string(query.Logic)+" "), arguments, nil
}

func trackFilterPredicate(filter metadata.TrackFilter, arguments *[]any) (string, error) {
	switch filter.Field {
	case metadata.TrackFilterTitle:
		return titleFilterPredicate(filter, arguments)
	case metadata.TrackFilterArtistID:
		return relationFilterPredicate(
			filter, arguments, "catalog.track_artists", "ta", "artist_id",
		)
	case metadata.TrackFilterAlbumID:
		return directUUIDFilterPredicate(filter, arguments, "t.album_id")
	case metadata.TrackFilterGenreID:
		return relationFilterPredicate(
			filter, arguments, "catalog.track_genres", "tg", "genre_id",
		)
	case metadata.TrackFilterMoodID:
		return relationFilterPredicate(
			filter, arguments, "catalog.track_moods", "tm", "mood_id",
		)
	case metadata.TrackFilterReleaseYear:
		return integerFilterPredicate(
			filter, arguments, "EXTRACT(YEAR FROM t.release_date)::integer",
		)
	case metadata.TrackFilterDurationMS:
		return integerFilterPredicate(filter, arguments, "t.duration_ms")
	case metadata.TrackFilterExplicit:
		if filter.Operator != metadata.TrackFilterEquals ||
			filter.Value.Kind != metadata.TrackFilterValueBoolean {
			return "", unsupportedTrackFilter(filter)
		}
		placeholder := appendFilterArgument(arguments, filter.Value.Boolean)
		return "t.explicit = " + placeholder + "::boolean", nil
	default:
		return "", unsupportedTrackFilter(filter)
	}
}

func titleFilterPredicate(filter metadata.TrackFilter, arguments *[]any) (string, error) {
	if filter.Value.Kind != metadata.TrackFilterValueText {
		return "", unsupportedTrackFilter(filter)
	}
	switch filter.Operator {
	case metadata.TrackFilterEquals:
		placeholder := appendFilterArgument(arguments, filter.Value.Text)
		return "lower(t.title) = lower(" + placeholder + "::text)", nil
	case metadata.TrackFilterContains:
		placeholder := appendFilterArgument(arguments, escapeLike(filter.Value.Text))
		return "t.title ILIKE ('%' || " + placeholder + "::text || '%') ESCAPE '\\'", nil
	default:
		return "", unsupportedTrackFilter(filter)
	}
}

func directUUIDFilterPredicate(
	filter metadata.TrackFilter,
	arguments *[]any,
	column string,
) (string, error) {
	switch {
	case filter.Operator == metadata.TrackFilterEquals &&
		filter.Value.Kind == metadata.TrackFilterValueUUID:
		placeholder := appendFilterArgument(arguments, filter.Value.UUID)
		return column + " = " + placeholder + "::uuid", nil
	case filter.Operator == metadata.TrackFilterIn &&
		filter.Value.Kind == metadata.TrackFilterValueUUIDList:
		placeholder := appendFilterArgument(arguments, filter.Value.UUIDs)
		return column + " = ANY(" + placeholder + "::uuid[])", nil
	default:
		return "", unsupportedTrackFilter(filter)
	}
}

func relationFilterPredicate(
	filter metadata.TrackFilter,
	arguments *[]any,
	table string,
	alias string,
	column string,
) (string, error) {
	relationPredicate, err := directUUIDFilterPredicate(
		filter, arguments, alias+"."+column,
	)
	if err != nil {
		return "", err
	}
	return "EXISTS (SELECT 1 FROM " + table + " AS " + alias +
		" WHERE " + alias + ".track_id = t.id AND " + relationPredicate + ")", nil
}

func integerFilterPredicate(
	filter metadata.TrackFilter,
	arguments *[]any,
	expression string,
) (string, error) {
	if filter.Operator == metadata.TrackFilterBetween &&
		filter.Value.Kind == metadata.TrackFilterValueIntegerRange {
		minimum := appendFilterArgument(arguments, filter.Value.IntegerRange.Minimum)
		maximum := appendFilterArgument(arguments, filter.Value.IntegerRange.Maximum)
		return expression + " BETWEEN " + minimum + "::integer AND " + maximum + "::integer", nil
	}
	if filter.Value.Kind != metadata.TrackFilterValueInteger {
		return "", unsupportedTrackFilter(filter)
	}
	operator := ""
	switch filter.Operator {
	case metadata.TrackFilterEquals:
		operator = "="
	case metadata.TrackFilterGreaterThan:
		operator = ">"
	case metadata.TrackFilterGreaterThanOrEqual:
		operator = ">="
	case metadata.TrackFilterLessThan:
		operator = "<"
	case metadata.TrackFilterLessThanOrEqual:
		operator = "<="
	default:
		return "", unsupportedTrackFilter(filter)
	}
	placeholder := appendFilterArgument(arguments, filter.Value.Integer)
	return expression + " " + operator + " " + placeholder + "::integer", nil
}

func appendFilterArgument(arguments *[]any, value any) string {
	*arguments = append(*arguments, value)
	return fmt.Sprintf("$%d", len(*arguments))
}

func unsupportedTrackFilter(filter metadata.TrackFilter) error {
	return fmt.Errorf(
		"unsupported track filter %q/%q/%q: %w",
		filter.Field, filter.Operator, filter.Value.Kind, metadata.ErrInvalid,
	)
}

func (repository *MetadataRepository) filteredTrackPage(
	ctx context.Context,
	filterSQL string,
	filterArguments []any,
	page metadata.PageRequest,
	orderBy string,
) (metadata.Page[metadata.Track], error) {
	var total int64
	if err := repository.database.QueryRow(
		ctx, `SELECT count(*) `+filterSQL, filterArguments...,
	).Scan(&total); err != nil {
		return metadata.Page[metadata.Track]{}, fmt.Errorf("count filtered tracks: %w", err)
	}
	limitPlaceholder := fmt.Sprintf("$%d", len(filterArguments)+1)
	offsetPlaceholder := fmt.Sprintf("$%d", len(filterArguments)+2)
	pageArguments := append(append([]any(nil), filterArguments...), page.Limit, page.Offset)
	rows, err := repository.database.Query(ctx, `
		SELECT
			t.id, t.album_id, t.title, t.duration_ms, t.track_number, t.disc_number, t.explicit,
			t.release_date, t.media_asset_id, t.content_id, t.created_at, t.updated_at
	`+filterSQL+` ORDER BY `+orderBy+` LIMIT `+limitPlaceholder+` OFFSET `+offsetPlaceholder,
		pageArguments...)
	if err != nil {
		return metadata.Page[metadata.Track]{}, fmt.Errorf("query filtered tracks: %w", err)
	}

	items := make([]metadata.Track, 0, page.Limit)
	ids := make([]uuid.UUID, 0, page.Limit)
	for rows.Next() {
		var track metadata.Track
		if err = scanTrack(rows, &track); err != nil {
			rows.Close()
			return metadata.Page[metadata.Track]{}, fmt.Errorf("scan track: %w", err)
		}
		items = append(items, track)
		ids = append(ids, track.ID)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return metadata.Page[metadata.Track]{}, fmt.Errorf("iterate tracks: %w", err)
	}

	if err = repository.populateTrackRelations(ctx, items, ids); err != nil {
		return metadata.Page[metadata.Track]{}, err
	}
	return metadata.Page[metadata.Track]{
		Items: items, Limit: page.Limit, Offset: page.Offset, Total: total,
	}, nil
}

func (repository *MetadataRepository) populateTrackRelations(
	ctx context.Context,
	items []metadata.Track,
	ids []uuid.UUID,
) error {
	artists, err := repository.trackArtistsByTrackIDs(ctx, ids)
	if err != nil {
		return fmt.Errorf("query track page artists: %w", err)
	}
	genres, err := repository.trackGenresByTrackIDs(ctx, ids)
	if err != nil {
		return fmt.Errorf("query track page genres: %w", err)
	}
	moods, err := repository.trackMoodsByTrackIDs(ctx, ids)
	if err != nil {
		return fmt.Errorf("query track page moods: %w", err)
	}
	for index := range items {
		trackID := items[index].ID
		items[index].Artists = artists[trackID]
		items[index].Genres = genres[trackID]
		items[index].Moods = moods[trackID]
		if items[index].Artists == nil {
			items[index].Artists = make([]metadata.TrackArtist, 0)
		}
		if items[index].Genres == nil {
			items[index].Genres = make([]metadata.Genre, 0)
		}
		if items[index].Moods == nil {
			items[index].Moods = make([]metadata.TrackMood, 0)
		}
	}
	return nil
}
