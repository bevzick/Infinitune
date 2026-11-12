import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { RouterProvider } from "react-router-dom";

import { router } from "./app/router";
import { PlayerProvider } from "./features/player/PlayerContext";
import { SessionProvider } from "./features/session/SessionContext";
import { ThemeProvider } from "./features/theme/ThemeContext";
import "./styles/global.css";
import "./styles/app.scss";

const root = document.getElementById("root");

if (!root) {
  throw new Error("Application root element was not found");
}

createRoot(root).render(
  <StrictMode>
    <ThemeProvider>
      <SessionProvider>
        <PlayerProvider>
          <RouterProvider router={router} />
        </PlayerProvider>
      </SessionProvider>
    </ThemeProvider>
  </StrictMode>,
);
