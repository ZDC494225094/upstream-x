import type {
  Account,
  AccountExportBundle,
  AccountImportResult,
  AccountOverview,
  CustomMenuLinksSettings,
  DailyUsageSummary,
  DashboardSummary,
  DetectionRun,
  LogCleanupResult,
  ScreenOverview,
  ScreenLayoutSettings,
  SettingsExportBundle,
  SettingsSnapshot,
  SMTPSettings,
} from "./types";

// When the frontend is hosted separately from the Go backend (e.g. on
// Vercel), set VITE_API_BASE to the backend origin at build time. Same-origin
// deployments leave it empty and keep using relative /api paths.
const API_BASE = ((import.meta as unknown as { env?: Record<string, string | undefined> }).env
  ?.VITE_API_BASE ?? "").replace(/\/+$/, "");

export interface ValidatedCredentials {
  access_token: string;
  refresh_token: string;
}

export interface ValidateAccountResult {
  snapshot: unknown;
  credentials?: ValidatedCredentials;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path.startsWith("/") ? `${API_BASE}${path}` : path, {
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
    ...init,
  });

  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(body.error || response.statusText);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json();
}

export const api = {
  request,
  login: (payload: { username: string; password: string }) =>
    request<{ username: string }>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  logout: () => request("/api/auth/logout", { method: "POST" }),
  me: () => request<{ username: string }>("/api/auth/me"),
  dashboard: (range = "3d") =>
    request<DashboardSummary>(`/api/dashboard/summary?range=${range}`),
  dailyUsage: (range = "30d") =>
    request<DailyUsageSummary>(`/api/dashboard/daily-usage?range=${range}`),
  dashboardScreen: () =>
    request<ScreenOverview>("/api/dashboard/screen"),
  refreshDashboardData: () =>
    request<ScreenOverview>("/api/dashboard/refresh", {
      method: "POST",
    }),
  getScreenLayout: () =>
    request<ScreenLayoutSettings>("/api/dashboard/screen-layout"),
  updateScreenLayout: (payload: ScreenLayoutSettings) =>
    request<ScreenLayoutSettings>("/api/dashboard/screen-layout", {
      method: "PUT",
      body: JSON.stringify(payload),
    }),
  listAccounts: async () => {
    const result = await request<{ items: Account[] }>("/api/accounts");
    return result.items;
  },
  exportAccounts: () =>
    request<AccountExportBundle>("/api/accounts/export"),
  importAccounts: (payload: unknown) =>
    request<AccountImportResult>("/api/accounts/import", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  createAccount: (payload: Record<string, unknown>) =>
    request<Account>("/api/accounts", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  updateAccount: (id: number, payload: Record<string, unknown>) =>
    request<Account>(`/api/accounts/${id}`, {
      method: "PUT",
      body: JSON.stringify(payload),
    }),
  deleteAccount: (id: number) =>
    request(`/api/accounts/${id}`, {
      method: "DELETE",
    }),
  validateNewAccount: (payload: Record<string, unknown>) =>
    request<ValidateAccountResult>("/api/accounts/validate", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  validateAccount: (id: number) =>
    request<ValidateAccountResult>(`/api/accounts/${id}/validate`, { method: "POST" }),
  checkAccount: (id: number) =>
    request(`/api/accounts/${id}/check`, { method: "POST" }),
  accountOverview: (id: number) =>
    request<AccountOverview>(`/api/accounts/${id}/overview`),
  balanceHistory: (id: number, range = "3d") =>
    request<{ items: Array<{ time: string; balance_usd: number }> }>(
      `/api/accounts/${id}/balance-history?range=${range}`,
    ),
  groupHistory: (
    id: number,
    range = "3d",
  ) =>
    request<{
      items: Array<{
        run_id: number;
        time: string;
        group_key: string;
        group_name: string;
        rate_multiplier: number;
      }>;
    }>(`/api/accounts/${id}/group-history?range=${range}`),
  runLogs: (id: number) =>
    request<{ items: DetectionRun[] }>(`/api/accounts/${id}/run-logs`),
  getSettings: () => request<SettingsSnapshot>("/api/settings"),
  exportSettings: () => request<SettingsExportBundle>("/api/settings/export"),
  importSettings: (payload: unknown) =>
    request<SettingsSnapshot>("/api/settings/import", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  getCustomMenuLinks: () =>
    request<CustomMenuLinksSettings>("/api/settings/custom-menu-links"),
  updateCustomMenuLinks: (payload: CustomMenuLinksSettings) =>
    request<CustomMenuLinksSettings>("/api/settings/custom-menu-links", {
      method: "PUT",
      body: JSON.stringify(payload),
    }),
  getSchedule: () => request<{ interval_minutes: number }>("/api/settings/schedule"),
  updateSchedule: (payload: { interval_minutes: number }) =>
    request<{ interval_minutes: number }>("/api/settings/schedule", {
      method: "PUT",
      body: JSON.stringify(payload),
    }),
  updateLogRetention: (payload: { log_retention_days: number }) =>
    request<{ log_retention_days: number }>("/api/settings/logs", {
      method: "PUT",
      body: JSON.stringify(payload),
    }),
  cleanupLogs: (payload: { log_retention_days: number }) =>
    request<LogCleanupResult>("/api/settings/logs/cleanup", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  updateSMTP: (payload: SMTPSettings) =>
    request<{ success: boolean }>("/api/settings/smtp", {
      method: "PUT",
      body: JSON.stringify(payload),
    }),
  sendSMTPTest: (payload: { subject?: string; body?: string }) =>
    request<{ success: boolean }>("/api/settings/smtp/test", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  updateProxy: (payload: { proxy_url: string }) =>
    request<{ success: boolean; proxy_url: string }>("/api/settings/proxy", {
      method: "PUT",
      body: JSON.stringify(payload),
    }),
  testProxy: (payload: { proxy_url: string }) =>
    request<{ ip: string; proxy_used: boolean }>("/api/settings/proxy/test", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
};
