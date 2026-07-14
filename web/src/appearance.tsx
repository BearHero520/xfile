import {
  createContext,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useState,
} from "react";

export type ThemePreference = "system" | "light" | "dark";
export type RadiusPreference = "compact" | "balanced" | "rounded";
export type MotionPreference = "reduced" | "standard" | "expressive";

interface AppearanceState {
  theme: ThemePreference;
  resolvedTheme: "light" | "dark";
  radius: RadiusPreference;
  motion: MotionPreference;
  setTheme: (value: ThemePreference) => void;
  setRadius: (value: RadiusPreference) => void;
  setMotion: (value: MotionPreference) => void;
}

const AppearanceContext = createContext<AppearanceState | null>(null);

function readPreference<T extends string>(
  key: string,
  allowed: readonly T[],
  fallback: T,
) {
  const value = localStorage.getItem(key) as T | null;
  return value && allowed.includes(value) ? value : fallback;
}

export function AppearanceProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [theme, setTheme] = useState<ThemePreference>(() =>
    readPreference("xfile-theme", ["system", "light", "dark"], "light"),
  );
  const [radius, setRadius] = useState<RadiusPreference>(() =>
    readPreference(
      "xfile-radius",
      ["compact", "balanced", "rounded"],
      "balanced",
    ),
  );
  const [motion, setMotion] = useState<MotionPreference>(() =>
    readPreference(
      "xfile-motion",
      ["reduced", "standard", "expressive"],
      "standard",
    ),
  );
  const [systemTheme, setSystemTheme] = useState<"light" | "dark">(() =>
    window.matchMedia("(prefers-color-scheme: dark)").matches
      ? "dark"
      : "light",
  );

  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const update = () => setSystemTheme(media.matches ? "dark" : "light");
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);

  const resolvedTheme = theme === "system" ? systemTheme : theme;

  useLayoutEffect(() => {
    const root = document.documentElement;
    root.dataset.theme = resolvedTheme;
    root.dataset.themePreference = theme;
    root.dataset.radius = radius;
    root.dataset.motion = motion;
    root.style.colorScheme = resolvedTheme;
    localStorage.setItem("xfile-theme", theme);
    localStorage.setItem("xfile-radius", radius);
    localStorage.setItem("xfile-motion", motion);
  }, [motion, radius, resolvedTheme, theme]);

  const value = useMemo(
    () => ({
      theme,
      resolvedTheme,
      radius,
      motion,
      setTheme,
      setRadius,
      setMotion,
    }),
    [motion, radius, resolvedTheme, theme],
  );

  return (
    <AppearanceContext.Provider value={value}>
      {children}
    </AppearanceContext.Provider>
  );
}

export function useAppearance() {
  const value = useContext(AppearanceContext);
  if (!value) {
    throw new Error("useAppearance must be used inside AppearanceProvider");
  }
  return value;
}
