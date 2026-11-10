package ranking

import "github.com/google/uuid"

type Mood struct {
	ID     uuid.UUID
	Weight float64
}

type Candidate struct {
	TrackID       uuid.UUID
	ContentID     uuid.UUID
	AlbumID       uuid.UUID
	ArtistIDs     []uuid.UUID
	GenreIDs      []uuid.UUID
	Moods         []Mood
	Explicit      bool
	DiscoveryRank int
}
