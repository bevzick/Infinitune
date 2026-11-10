import { describe, expect, it } from "vitest";

import { readCatalogSearch, searchTrackFilters } from "./searchState";

describe("catalog URL state", () => {
  it("reads filters and pagination from a shareable URL", () => {
    const state = readCatalogSearch(
      new URLSearchParams(
        "q=signal&artist=artist-id&album=album-id&genre=genre-id&mood=mood-id&year=2026&explicit=false&sort=title_desc&page=3",
      ),
    );

    expect(state).toMatchObject({
      query: "signal",
      releaseYear: "2026",
      explicit: "false",
      sort: "title_desc",
      page: 3,
    });
    expect(searchTrackFilters(state)).toEqual({
      title: "signal",
      artistId: "artist-id",
      albumId: "album-id",
      genreId: "genre-id",
      moodId: "mood-id",
      releaseYear: 2026,
      explicit: false,
      sort: "title_desc",
      limit: 10,
      offset: 20,
    });
  });

  it("falls back safely for unsupported URL values", () => {
    const state = readCatalogSearch(
      new URLSearchParams("year=10000&explicit=yes&sort=random&page=-2"),
    );

    expect(state.releaseYear).toBe("");
    expect(state.explicit).toBe("");
    expect(state.sort).toBe("newest");
    expect(state.page).toBe(1);
  });
});
