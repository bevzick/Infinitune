import { createBrowserRouter } from "react-router-dom";
import { GuestRoute, ProtectedRoute } from "../features/session/routeGuards";
import { DiscoverPage } from "../pages/DiscoverPage";
import { LibraryPage } from "../pages/LibraryPage";
import { LoginPage } from "../pages/LoginPage";
import { NotFoundPage } from "../pages/NotFoundPage";
import { RegisterPage } from "../pages/RegisterPage";
import { RouteErrorPage } from "../pages/RouteErrorPage";
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
      {
        element: <ProtectedRoute />,
        children: [{ path: "library", element: <LibraryPage /> }],
      },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
]);
