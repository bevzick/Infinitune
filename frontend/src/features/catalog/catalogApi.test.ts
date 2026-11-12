import { describe, expect, it } from "vitest";

import {
  artistDiscographyPath,
  catalogDetailPath,
  catalogPagePath,
  trackLyricsPath,
  trackPagePath,
} from "./catalogApi";

describe("catalog paths", () => {
  it("builds a paginated collection path", () => {
    expect(catalogPagePath("albums", 12, 24)).toBe(
      "/v1/catalog/albums?limit=12&offset=24",
    );
  });

  it("maps every supported track filter to the REST contract", () => {
    const path = trackPagePath({
      title: "  night drive  ",
      artistId: "artist-id",
      albumId: "album-id",
      genreId: "genre-id",
      moodId: "mood-id",
      releaseYear: 2026,
      explicit: false,
      sort: "title_asc",
      limit: 10,
      offset: 20,
    });

    expect(path).toBe(
      "/v1/catalog/tracks?title=night+drive&artist_id=artist-id&album_id=album-id&genre_id=genre-id&mood_id=mood-id&release_year=2026&explicit=false&sort=title_asc&limit=10&offset=20",
    );
  });

  it("builds the public lyrics path", () => {
    expect(trackLyricsPath("track/id")).toBe(
      "/v1/catalog/tracks/track%2Fid/lyrics",
    );
  });

  it("escapes catalog detail identifiers", () => {
    expect(catalogDetailPath("tracks", "track/id")).toBe(
      "/v1/catalog/tracks/track%2Fid",
    );
  });

  it("builds an escaped artist discography path", () => {
    expect(artistDiscographyPath("artist/id", 25, 50)).toBe(
      "/v1/catalog/artists/artist%2Fid/discography?limit=25&offset=50",
    );
  });
});
