import { describe, expect, it } from "vitest";

import { catalogPagePath, trackPagePath } from "./catalogApi";

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
});
