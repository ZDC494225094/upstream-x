import { Menu, LayoutDashboard, LogOut, ServerCog, Settings, Bell, History, RefreshCw, Monitor, Moon, Sun, Link2 } from "lucide-react";
import { Link, NavLink, useLocation } from "react-router-dom";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { useCustomMenuLinks } from "@/hooks/use-custom-menu-links";
import { usePageRefresh } from "@/hooks/use-page-refresh";
import { useTheme } from "@/hooks/use-theme";

interface AppShellProps {
  username: string;
  onLogout: () => Promise<void>;
  children: React.ReactNode;
}

const navItems = [
  { to: "/", label: "总览", icon: LayoutDashboard },
  { to: "/screen", label: "账号大屏", icon: Monitor },
  { to: "/accounts", label: "账号管理", icon: ServerCog },
  { to: "/runs", label: "检测任务", icon: History },
  { to: "/alerts", label: "告警变更", icon: Bell },
];

const settingsNavItem = { to: "/settings", label: "系统设置", icon: Settings };

export function AppShell({ username, onLogout, children }: AppShellProps) {
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const location = useLocation();
  const { links: customMenuLinks } = useCustomMenuLinks();
  const { triggerRefresh } = usePageRefresh();
  const { theme, toggleTheme } = useTheme();
  const SettingsIcon = settingsNavItem.icon;

  async function handleRefresh() {
    setRefreshing(true);
    try {
      triggerRefresh();
      await new Promise((resolve) => window.setTimeout(resolve, 300));
    } finally {
      setRefreshing(false);
    }
  }

  const sidebarContent = (isMobile = false) => (
    <>
      <div className="flex h-16 items-center border-b border-white/10 px-6">
        <Link to="/" className="flex items-center gap-3" onClick={() => isMobile && setMobileMenuOpen(false)}>
          <span className="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-cyan-400 to-blue-600 shadow-[0_0_24px_rgba(34,211,238,0.45)]">
            <img src="/favicon.png" alt="X" className="h-6 w-6 rounded" />
          </span>
          <div>
            <div className="text-sm font-semibold tracking-wide text-white">Upstream X</div>
            <div className="text-xs text-slate-400">上游监控控制台</div>
          </div>
        </Link>
      </div>
      <div className="p-4">
        <nav className="space-y-1">
          {navItems.map((item) => {
            const Icon = item.icon;
            return (
              <NavLink
                key={item.to}
                to={item.to}
                onClick={() => isMobile && setMobileMenuOpen(false)}
                className={({ isActive }) =>
                  `flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition ${
                    isActive ? "bg-cyan-400/10 text-cyan-300 shadow-[inset_0_0_0_1px_rgba(34,211,238,0.25)]" : "text-slate-400 hover:bg-white/5 hover:text-slate-100"
                  }`
                }
              >
                <Icon className="h-4 w-4" />
                {item.label}
              </NavLink>
            );
          })}
          <NavLink
            to={settingsNavItem.to}
            onClick={() => isMobile && setMobileMenuOpen(false)}
            className={({ isActive }) =>
              `flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition ${
                isActive ? "bg-cyan-400/10 text-cyan-300 shadow-[inset_0_0_0_1px_rgba(34,211,238,0.25)]" : "text-slate-400 hover:bg-white/5 hover:text-slate-100"
              }`
            }
          >
            <SettingsIcon className="h-4 w-4" />
            {settingsNavItem.label}
          </NavLink>
          {customMenuLinks.length ? (
            <div className="px-3 pb-1 pt-3">
              <div className="border-t" />
              <div className="pt-3 text-[11px] font-medium tracking-[0.12em] text-slate-500">自定义菜单</div>
            </div>
          ) : null}
          {customMenuLinks.map((item) => {
            const to = item.embed_iframe ? `/custom-menu-links/${item.id}` : item.url;
            const isInternalLink = item.url.startsWith("/");
            const isActive = item.embed_iframe
              ? location.pathname === `/custom-menu-links/${item.id}`
              : isInternalLink
                ? location.pathname === item.url
                : false;

            if (!item.embed_iframe && !isInternalLink) {
              return (
                <a
                  key={item.id}
                  href={item.url}
                  target="_blank"
                  rel="noreferrer"
                  onClick={() => isMobile && setMobileMenuOpen(false)}
                  className="flex items-center gap-3 rounded-lg px-3 py-2 text-sm text-slate-400 transition hover:bg-white/5 hover:text-slate-100"
                >
                  <Link2 className="h-4 w-4" />
                  {item.title}
                </a>
              );
            }

            return (
              <NavLink
                key={item.id}
                to={to}
                onClick={() => isMobile && setMobileMenuOpen(false)}
                className={() =>
                  `flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition ${
                    isActive ? "bg-cyan-400/10 text-cyan-300 shadow-[inset_0_0_0_1px_rgba(34,211,238,0.25)]" : "text-slate-400 hover:bg-white/5 hover:text-slate-100"
                  }`
                }
              >
                <Link2 className="h-4 w-4" />
                {item.title}
              </NavLink>
            );
          })}
        </nav>
      </div>
    </>
  );

  return (
    <div className="min-h-screen md:h-screen md:overflow-hidden">
      <div className="min-h-screen md:grid md:h-full md:grid-cols-[240px_minmax(0,1fr)]">
        <aside className="hidden border-r border-white/10 bg-[#070D1A] bg-gradient-to-b from-[#0A1322] to-[#05080F] text-slate-200 md:sticky md:top-0 md:block md:h-screen md:overflow-y-auto">
          {sidebarContent()}
        </aside>
        <div className="flex min-h-screen flex-col md:h-screen md:min-h-0 md:overflow-hidden">
          <header className="sticky top-0 z-20 flex h-16 shrink-0 items-center justify-between border-b bg-background/75 px-3 backdrop-blur-xl md:px-6">
            <div className="flex min-w-0 items-center gap-2.5 md:gap-3">
              <Button
                variant="ghost"
                size="sm"
                className="px-2 md:hidden"
                onClick={() => setMobileMenuOpen(true)}
              >
                <Menu className="h-5 w-5" />
              </Button>
              <div className="min-w-0">
                <div className="text-sm font-medium">管理后台</div>
                <div className="hidden truncate text-xs text-muted-foreground sm:block">欢迎回来，{username}</div>
              </div>
            </div>
            <div className="flex shrink-0 items-center gap-1.5 md:gap-2">
              <Button
                variant="outline"
                size="sm"
                className="px-2.5 md:px-3"
                onClick={toggleTheme}
                aria-label={theme === "dark" ? "切换到浅色模式" : "切换到夜间模式"}
                title={theme === "dark" ? "浅色模式" : "夜间模式"}
              >
                {theme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="px-2.5 md:px-3"
                onClick={() => void handleRefresh()}
                disabled={refreshing}
                aria-label={refreshing ? "刷新中" : "刷新数据"}
                title={refreshing ? "刷新中" : "刷新数据"}
              >
                <RefreshCw className={`h-4 w-4 ${refreshing ? "animate-spin" : ""}`} />
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="px-2.5 md:px-3"
                onClick={onLogout}
                aria-label="退出登录"
                title="退出登录"
              >
                <LogOut className="h-4 w-4 md:mr-2" />
                <span className="hidden md:inline">退出登录</span>
              </Button>
            </div>
          </header>
          <main className="flex-1 p-4 pb-8 md:min-h-0 md:overflow-y-auto md:p-6">
            <div className="mx-auto max-w-7xl space-y-6">{children}</div>
          </main>
        </div>
      </div>

      <Sheet open={mobileMenuOpen} onOpenChange={setMobileMenuOpen}>
        <SheetContent className="border-white/10 bg-[#070D1A] p-0 text-slate-200 md:hidden">
          <SheetHeader className="sr-only">
            <SheetTitle>导航菜单</SheetTitle>
            <SheetDescription>切换总览、账号管理、检测任务和系统设置页面。</SheetDescription>
          </SheetHeader>
          {sidebarContent(true)}
        </SheetContent>
      </Sheet>
    </div>
  );
}
