import { useSearchParams } from "react-router-dom";

import {
  AlbumGrid,
  ArtistGrid,
  CatalogEmpty,
  CatalogError,
  CatalogLoading,
  CatalogPagination,
} from "../features/catalog/CatalogViews";
import {
  type Album,
  type Artist,
  catalogPagePath,
} from "../features/catalog/catalogApi";
import { useCatalogPage } from "../features/catalog/useCatalogPage";

const pageSize = 12;

export function ArtistsPage() {
  return <CollectionPage kind="artists" />;
}

export function AlbumsPage() {
  return <CollectionPage kind="albums" />;
}

function CollectionPage({ kind }: { kind: "artists" | "albums" }) {
  const [parameters] = useSearchParams();
  const page = positivePage(parameters.get("page"));
  const path = catalogPagePath(kind, pageSize, (page - 1) * pageSize);
  const result = useCatalogPage<Artist | Album>(path);
  const title = kind === "artists" ? "Artists" : "Albums & EPs";

  return (
    <section className="collection-page">
      <header className="catalog-page-heading">
        <span className="hero-kicker">Catalog collection</span>
        <h1>{title}</h1>
        <p>
          {kind === "artists"
            ? "Meet every voice currently published on Infinitune."
            : "Browse releases from newest to oldest."}
        </p>
      </header>
      {result.status === "loading" ? <CatalogLoading label={kind} /> : null}
      {result.status === "error" ? (
        <CatalogError message={result.message} onRetry={result.retry} />
      ) : null}
      {result.status === "ready" && !result.page.items.length ? (
        <>
          <CatalogEmpty>No {kind} are available on this page</CatalogEmpty>
          <CatalogPagination
            page={page}
            pageSize={pageSize}
            total={result.page.pagination.total}
            pageUrl={(next) => `/${kind}${next > 1 ? `?page=${next}` : ""}`}
          />
        </>
      ) : null}
      {result.status === "ready" && result.page.items.length ? (
        <>
          {kind === "artists" ? (
            <ArtistGrid artists={result.page.items as Artist[]} />
          ) : (
            <AlbumGrid albums={result.page.items as Album[]} />
          )}
          <CatalogPagination
            page={page}
            pageSize={pageSize}
            total={result.page.pagination.total}
            pageUrl={(next) => `/${kind}${next > 1 ? `?page=${next}` : ""}`}
          />
        </>
      ) : null}
    </section>
  );
}

function positivePage(value: string | null) {
  const page = Number(value ?? "1");
  return Number.isInteger(page) && page > 0 ? page : 1;
}
