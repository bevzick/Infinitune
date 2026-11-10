import { createBrowserRouter } from "react-router-dom";
import { GuestRoute, ProtectedRoute } from "../features/session/routeGuards";
import { AlbumsPage, ArtistsPage } from "../pages/CatalogCollectionPage";
import { DiscoverPage } from "../pages/DiscoverPage";
import { LibraryPage } from "../pages/LibraryPage";
import { LoginPage } from "../pages/LoginPage";
import { NotFoundPage } from "../pages/NotFoundPage";
import { PlaylistPage } from "../pages/PlaylistPage";
import { RegisterPage } from "../pages/RegisterPage";
import { RouteErrorPage } from "../pages/RouteErrorPage";
import { SearchPage } from "../pages/SearchPage";
import { AppLayout } from "./AppLayout";

export const router = createBrowserRouter([
  {
    element: <GuestRoute />,
    children: [
      { path: "/login", element: <LoginPage /> },
      { path: "/register", element: <RegisterPage /> },
    ],
  },
  {
    path: "/",
    element: <AppLayout />,
    errorElement: <RouteErrorPage />,
    children: [
      { index: true, element: <DiscoverPage /> },
      { path: "search", element: <SearchPage /> },
      { path: "artists", element: <ArtistsPage /> },
      { path: "albums", element: <AlbumsPage /> },
      {
        element: <ProtectedRoute />,
        children: [
          { path: "library", element: <LibraryPage /> },
          { path: "library/:playlistId", element: <PlaylistPage /> },
        ],
      },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
]);
