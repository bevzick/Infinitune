import {
  createContext,
  type PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

export type Theme = "light" | "dark";

type ThemeContextValue = {
  theme: Theme;
  setTheme(theme: Theme): void;
  toggleTheme(): void;
};

const ThemeContext = createContext<ThemeContextValue | null>(null);
const storageKey = "infinitune-theme";

export function ThemeProvider({ children }: PropsWithChildren) {
  const [theme, setThemeState] = useState<Theme>(initialTheme);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
    document
      .querySelector('meta[name="theme-color"]')
      ?.setAttribute("content", theme === "dark" ? "#1d1d26" : "#f5f2fb");
    window.localStorage.setItem(storageKey, theme);
  }, [theme]);

  const setTheme = useCallback((next: Theme) => setThemeState(next), []);
  const toggleTheme = useCallback(
    () => setThemeState((current) => (current === "dark" ? "light" : "dark")),
    [],
  );
  const value = useMemo(
    () => ({ theme, setTheme, toggleTheme }),
    [theme, setTheme, toggleTheme],
  );

  return (
    <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
  );
}

export function useTheme() {
  const value = useContext(ThemeContext);
  if (!value) throw new Error("useTheme must be used inside ThemeProvider");
  return value;
}

function initialTheme(): Theme {
  const stored = window.localStorage.getItem(storageKey);
  const theme =
    stored === "light" || stored === "dark"
      ? stored
      : window.matchMedia?.("(prefers-color-scheme: light)").matches
        ? "light"
        : "dark";
  document.documentElement.dataset.theme = theme;
  return theme;
}
