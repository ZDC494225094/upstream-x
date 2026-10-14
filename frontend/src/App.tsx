import { Route, Routes, useNavigate } from "react-router-dom";

import { AppShell } from "@/layouts/app-shell";
import { DashboardPage } from "@/pages/dashboard-page";
import { AccountsPage } from "@/pages/accounts-page";
import { AccountDetailPage } from "@/pages/account-detail-page";
import { AlertsPage } from "@/pages/alerts-page";
import { LoginPage } from "@/pages/login-page";
import { RunsPage } from "@/pages/runs-page";
import { ScreenPage } from "@/pages/screen-page";
import { SettingsPage } from "@/pages/settings-page";
import { CustomMenuFramePage } from "@/pages/custom-menu-frame-page";
import { api } from "@/lib/api";
import { useAuth } from "@/hooks/use-auth";
import { CustomMenuLinksProvider } from "@/hooks/use-custom-menu-links";
import { PageRefreshProvider } from "@/hooks/use-page-refresh";
import { ThemeProvider } from "@/hooks/use-theme";
import { ToastProvider } from "@/hooks/use-toast";

export default function App() {
  const { username, setUsername, loading } = useAuth();
  const navigate = useNavigate();

  async function handleLogout() {
    await api.logout();
    setUsername(null);
    navigate("/");
  }

  if (loading) {
    return <div className="flex min-h-screen items-center justify-center text-sm text-muted-foreground">加载中...</div>;
  }

  if (!username) {
    return <LoginPage onSuccess={setUsername} />;
  }

  return (
    <ThemeProvider>
      <ToastProvider>
        <PageRefreshProvider>
          <CustomMenuLinksProvider>
            <AppShell username={username} onLogout={handleLogout}>
              <Routes>
                <Route path="/" element={<DashboardPage />} />
                <Route path="/screen" element={<ScreenPage />} />
                <Route path="/accounts" element={<AccountsPage />} />
                <Route path="/accounts/:id" element={<AccountDetailPage />} />
                <Route path="/runs" element={<RunsPage />} />
                <Route path="/alerts" element={<AlertsPage />} />
                <Route path="/settings" element={<SettingsPage />} />
                <Route path="/custom-menu-links/:id" element={<CustomMenuFramePage />} />
              </Routes>
            </AppShell>
          </CustomMenuLinksProvider>
        </PageRefreshProvider>
      </ToastProvider>
    </ThemeProvider>
  );
}
