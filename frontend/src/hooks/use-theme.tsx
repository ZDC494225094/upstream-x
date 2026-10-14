import { createContext, useContext, useEffect, useMemo, useState } from "react";

type Theme = "light" | "dark";

interface ThemeContextValue {
  theme: Theme;
  toggleTheme: () => void;
}

const storageKey = "upstream-x-theme";
const ThemeContext = createContext<ThemeContextValue | null>(null);

function applyTheme(theme: Theme) {
  document.documentElement.classList.toggle("dark", theme === "dark");
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setTheme] = useState<Theme>("dark");

  useEffect(() => {
    const stored = window.localStorage.getItem(storageKey);
    // Dark is the default theme for the console; an explicit stored
    // preference always wins.
    const nextTheme: Theme = stored === "light" ? "light" : "dark";
    setTheme(nextTheme);
    applyTheme(nextTheme);
  }, []);

  const value = useMemo<ThemeContextValue>(() => ({
    theme,
    toggleTheme: () => {
      setTheme((current) => {
        const nextTheme: Theme = current === "dark" ? "light" : "dark";
        window.localStorage.setItem(storageKey, nextTheme);
        applyTheme(nextTheme);
        return nextTheme;
      });
    },
  }), [theme]);

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  const context = useContext(ThemeContext);
  if (!context) {
    throw new Error("useTheme must be used within ThemeProvider");
  }
  return context;
}
