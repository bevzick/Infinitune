import { type ElementType, lazy, Suspense } from "react";
import { createBrowserRouter } from "react-router-dom";

import {
  AdminRoute,
  GuestRoute,
  ProtectedRoute,
} from "../features/session/routeGuards";
import { RouteLoading } from "../shared/ui/RouteLoading";
import { AppLayout } from "./AppLayout";

const LoginPage = lazy(async () => ({
  default: (await import("../pages/LoginPage")).LoginPage,
}));
const RegisterPage = lazy(async () => ({
  default: (await import("../pages/RegisterPage")).RegisterPage,
}));
const DiscoverPage = lazy(async () => ({
  default: (await import("../pages/DiscoverPage")).DiscoverPage,
}));
const SearchPage = lazy(async () => ({
  default: (await import("../pages/SearchPage")).SearchPage,
}));
const ArtistsPage = lazy(async () => ({
  default: (await import("../pages/CatalogCollectionPage")).ArtistsPage,
}));
const AlbumsPage = lazy(async () => ({
  default: (await import("../pages/CatalogCollectionPage")).AlbumsPage,
}));
const ArtistPage = lazy(async () => ({
  default: (await import("../pages/CatalogDetailPage")).ArtistPage,
}));
const AlbumPage = lazy(async () => ({
  default: (await import("../pages/CatalogDetailPage")).AlbumPage,
}));
const TrackPage = lazy(async () => ({
  default: (await import("../pages/CatalogDetailPage")).TrackPage,
}));
const LibraryPage = lazy(async () => ({
  default: (await import("../pages/LibraryPage")).LibraryPage,
}));
const PlaylistPage = lazy(async () => ({
  default: (await import("../pages/PlaylistPage")).PlaylistPage,
}));
const ProfilePage = lazy(async () => ({
  default: (await import("../pages/ProfilePage")).ProfilePage,
}));
const MusicImportPage = lazy(async () => ({
  default: (await import("../pages/MusicImportPage")).MusicImportPage,
}));
const CreatorReleasesPage = lazy(async () => ({
  default: (await import("../pages/CreatorReleasesPage")).CreatorReleasesPage,
}));
const AdminPage = lazy(async () => ({
  default: (await import("../pages/AdminPage")).AdminPage,
}));
const NotFoundPage = lazy(async () => ({
  default: (await import("../pages/NotFoundPage")).NotFoundPage,
}));
const RouteErrorPage = lazy(async () => ({
  default: (await import("../pages/RouteErrorPage")).RouteErrorPage,
}));

export const router = createBrowserRouter([
  {
    element: <GuestRoute />,
    children: [
      { path: "/login", element: deferred(LoginPage, "sign in", true) },
      {
        path: "/register",
        element: deferred(RegisterPage, "registration", true),
      },
    ],
  },
  {
    path: "/",
    element: <AppLayout />,
    errorElement: deferred(RouteErrorPage, "error details"),
    children: [
      { index: true, element: deferred(DiscoverPage, "discoveries") },
      { path: "search", element: deferred(SearchPage, "search") },
      { path: "artists", element: deferred(ArtistsPage, "artists") },
      {
        path: "artists/:artistId",
        element: deferred(ArtistPage, "artist"),
      },
      { path: "albums", element: deferred(AlbumsPage, "albums") },
      { path: "albums/:albumId", element: deferred(AlbumPage, "album") },
      { path: "tracks/:trackId", element: deferred(TrackPage, "track") },
      {
        element: <ProtectedRoute />,
        children: [
          { path: "library", element: deferred(LibraryPage, "library") },
          {
            path: "library/:playlistId",
            element: deferred(PlaylistPage, "playlist"),
          },
          { path: "profile", element: deferred(ProfilePage, "profile") },
          {
            path: "creator/releases",
            element: deferred(CreatorReleasesPage, "release drafts"),
          },
          {
            path: "creator/import",
            element: deferred(MusicImportPage, "music import"),
          },
        ],
      },
      {
        element: <AdminRoute />,
        children: [{ path: "admin", element: deferred(AdminPage, "admin") }],
      },
      { path: "*", element: deferred(NotFoundPage, "page") },
    ],
  },
]);

function deferred(Component: ElementType, label: string, fullPage = false) {
  return (
    <Suspense fallback={<RouteLoading label={label} fullPage={fullPage} />}>
      <Component />
    </Suspense>
  );
}
