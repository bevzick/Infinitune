import { createBrowserRouter } from "react-router-dom";
import { DiscoverPage } from "../pages/DiscoverPage";
import { NotFoundPage } from "../pages/NotFoundPage";
import { RouteErrorPage } from "../pages/RouteErrorPage";
import { AppLayout } from "./AppLayout";

export const router = createBrowserRouter([
  {
    path: "/",
    element: <AppLayout />,
    errorElement: <RouteErrorPage />,
    children: [
      { index: true, element: <DiscoverPage /> },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
]);
