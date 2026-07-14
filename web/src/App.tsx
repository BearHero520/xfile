import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import Shell from "./components/Shell";
import { Loading } from "./components/ui";
import { useApp } from "./state";
import AccountsPage from "./pages/AccountsPage";
import AuditPage from "./pages/AuditPage";
import DeliveryPage from "./pages/DeliveryPage";
import ExplorerPage from "./pages/ExplorerPage";
import FavoritesPage from "./pages/FavoritesPage";
import LoginPage from "./pages/LoginPage";
import InsightsPage from "./pages/InsightsPage";
import SearchPage from "./pages/SearchPage";
import SettingsPage from "./pages/SettingsPage";
import SharePage from "./pages/SharePage";
import SharesPage from "./pages/SharesPage";
import StoragePage from "./pages/StoragePage";

function HomeRoute() {
  const { session, loading } = useApp();
  if (loading)
    return (
      <div className="fullscreen-center">
        <Loading label="正在初始化 XFile" />
      </div>
    );
  return <ExplorerPage publicMode={!session?.authenticated} />;
}

function ProtectedShell() {
  const { session, loading } = useApp();
  const location = useLocation();
  if (loading)
    return (
      <div className="fullscreen-center">
        <Loading label="正在初始化 XFile" />
      </div>
    );
  if (!session?.authenticated)
    return (
      <Navigate
        to={`/login?redirect=${encodeURIComponent(location.pathname + location.search)}`}
        replace
      />
    );
  return <Shell />;
}

function AdminOnly({ children }: { children: React.ReactNode }) {
  const { session } = useApp();
  if (session?.user?.role !== "super_admin") return <Navigate to="/" replace />;
  return children;
}

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<HomeRoute />} />
      <Route path="/login" element={<LoginPage />} />
      <Route path="/share/:token" element={<SharePage />} />
      <Route element={<ProtectedShell />}>
        <Route path="search" element={<SearchPage />} />
        <Route path="storage" element={<StoragePage />} />
        <Route path="shares" element={<SharesPage />} />
        <Route path="delivery" element={<DeliveryPage />} />
        <Route path="favorites" element={<FavoritesPage />} />
        <Route
          path="accounts"
          element={
            <AdminOnly>
              <AccountsPage />
            </AdminOnly>
          }
        />
        <Route path="audit" element={<AuditPage />} />
        <Route path="insights" element={<InsightsPage />} />
        <Route
          path="settings/site"
          element={
            <AdminOnly>
              <SettingsPage section="site" />
            </AdminOnly>
          }
        />
        <Route
          path="settings/view"
          element={
            <AdminOnly>
              <SettingsPage section="view" />
            </AdminOnly>
          }
        />
        <Route
          path="settings/webdav"
          element={
            <AdminOnly>
              <SettingsPage section="webdav" />
            </AdminOnly>
          }
        />
        {[
          "link",
          "upload",
          "visibility",
          "user-rules",
          "security",
          "access",
        ].map((section) => (
          <Route
            key={section}
            path={`settings/${section}`}
            element={
              <AdminOnly>
                <SettingsPage
                  section={
                    section as import("./pages/SettingsPage").SettingsSection
                  }
                />
              </AdminOnly>
            }
          />
        ))}
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
