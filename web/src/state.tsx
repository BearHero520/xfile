import type { AuthSession, PublicSite } from "./types";
import { Dismiss16Regular } from "@fluentui/react-icons";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { api } from "./api";

interface AppState {
  session: AuthSession | null;
  site: PublicSite | null;
  loading: boolean;
  refresh: () => Promise<void>;
}

const AppContext = createContext<AppState | null>(null);

function applySiteBranding(site: PublicSite) {
  const preset = site.preferences.themePreset || "ocean";
  document.documentElement.dataset.brandTheme = preset;
  document.title = site.siteName || "XFile";

  const themeColors: Record<string, string> = {
    ocean: "#087cf0",
    violet: "#7c3aed",
    emerald: "#059669",
    sunset: "#ea580c",
    graphite: "#475569",
    sky: "#0891b2",
    rose: "#db2777",
    sunflower: "#d97706",
  };
  const themeColor = document.querySelector<HTMLMetaElement>(
    'meta[name="theme-color"]',
  );
  if (themeColor) themeColor.content = themeColors[preset] || themeColors.ocean;

  let favicon = document.querySelector<HTMLLinkElement>('link[rel~="icon"]');
  if (!favicon) {
    favicon = document.createElement("link");
    favicon.rel = "icon";
    document.head.appendChild(favicon);
  }
  favicon.href = site.preferences.brandFaviconUrl || "/favicon.svg";
}

export function AppProvider({ children }: { children: React.ReactNode }) {
  const [session, setSession] = useState<AuthSession | null>(null);
  const [site, setSite] = useState<PublicSite | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    const [nextSession, nextSite] = await Promise.all([
      api.session(),
      api.site(),
    ]);
    applySiteBranding(nextSite);
    setSession(nextSession);
    setSite(nextSite);
  }, []);

  useEffect(() => {
    refresh().finally(() => setLoading(false));
  }, [refresh]);

  useEffect(() => {
    const syncSiteBranding = () => {
      if (document.visibilityState === "hidden") return;
      void api
        .site()
        .then((nextSite) => {
          applySiteBranding(nextSite);
          setSite(nextSite);
        })
        .catch(() => undefined);
    };
    window.addEventListener("focus", syncSiteBranding);
    document.addEventListener("visibilitychange", syncSiteBranding);
    return () => {
      window.removeEventListener("focus", syncSiteBranding);
      document.removeEventListener("visibilitychange", syncSiteBranding);
    };
  }, []);

  const value = useMemo(
    () => ({ session, site, loading, refresh }),
    [session, site, loading, refresh],
  );
  return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
}

export function useApp() {
  const value = useContext(AppContext);
  if (!value) throw new Error("useApp must be used inside AppProvider");
  return value;
}

type ToastKind = "success" | "error" | "info";
interface ToastItem {
  id: number;
  message: string;
  kind: ToastKind;
}
interface ToastState {
  show: (message: string, kind?: ToastKind) => void;
}
const ToastContext = createContext<ToastState | null>(null);

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);
  const dismiss = useCallback((id: number) => {
    setItems((current) => current.filter((item) => item.id !== id));
  }, []);
  const show = useCallback((message: string, kind: ToastKind = "info") => {
    const id = Date.now() + Math.random();
    setItems((current) => [...current, { id, message, kind }]);
    window.setTimeout(() => dismiss(id), 3200);
  }, [dismiss]);
  return (
    <ToastContext.Provider value={{ show }}>
      {children}
      <div className="toast-stack" aria-live="polite">
        {items.map((item) => (
          <div className={`toast toast-${item.kind}`} key={item.id}>
            <span className="toast-message">{item.message}</span>
            <button
              type="button"
              className="toast-close"
              aria-label="关闭提示"
              title="关闭提示"
              onClick={() => dismiss(item.id)}
            >
              <Dismiss16Regular />
              <span>关闭</span>
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const value = useContext(ToastContext);
  if (!value) throw new Error("useToast must be used inside ToastProvider");
  return value;
}
