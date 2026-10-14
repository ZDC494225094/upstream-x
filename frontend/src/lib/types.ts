export type UpstreamType = "sub2api" | "newapi";
export type Sub2APIAuthMode = "refresh_token" | "password";
export type NewAPIAuthMode = "manual";

export interface Account {
  id: number;
  created_at?: string;
  updated_at?: string;
  name: string;
  remark?: string;
  type: UpstreamType;
  base_url: string;
  sub2api_auth_mode?: Sub2APIAuthMode;
  sub2api_login_body_template?: string;
  custom_user_agent?: string;
  newapi_auth_mode?: NewAPIAuthMode;
  sub2api_email?: string;
  new_api_user?: string;
  low_balance_threshold_usd: number;
  enabled: boolean;
  last_status?: string;
  last_error?: string;
  last_checked_at?: string | null;
  last_balance_usd?: number | null;
}

export interface AccountTransferItem {
  id?: number;
  name: string;
  remark?: string;
  type: UpstreamType;
  base_url: string;
  sub2api_auth_mode?: Sub2APIAuthMode;
  sub2api_login_body_template?: string;
  custom_user_agent?: string;
  newapi_auth_mode?: NewAPIAuthMode;
  bearer_token: string;
  refresh_token?: string;
  sub2api_email?: string;
  sub2api_password?: string;
  new_api_user: string;
  low_balance_threshold_usd: number;
  enabled: boolean;
}

export interface AccountExportBundle {
  version?: string;
  exported_at?: string;
  accounts: AccountTransferItem[];
}

export interface AccountImportError {
  index: number;
  id?: number;
  name: string;
  message: string;
}

export interface AccountImportResult {
  created: number;
  updated: number;
  failed: number;
  errors: AccountImportError[];
}

export interface DetectionRun {
  id: number;
  account_id: number;
  account_name?: string;
  account_type?: UpstreamType;
  status: string;
  error_message?: string;
  balance_usd?: number | null;
  groups_count: number;
  tokens_count: number;
  triggered_by: string;
  started_at: string;
  completed_at: string;
  duration_millis: number;
}

export interface AlertEvent {
  id: number;
  account_id: number;
  account_name?: string;
  account_type?: UpstreamType;
  type: string;
  title: string;
  message: string;
  severity: string;
  group_key?: string;
  prev_value?: string;
  current_value?: string;
  created_at: string;
}

export interface BalancePoint {
  time: string;
  balance_usd: number;
}

export interface DashboardSummary {
  accounts_total: number;
  accounts_enabled: number;
  low_balance_accounts: number;
  failed_accounts: number;
  latest_runs: DetectionRun[];
  recent_alerts: AlertEvent[];
  account_balances: Array<{
    account_id: number;
    account_name: string;
    type: UpstreamType;
    points: BalancePoint[];
  }>;
}

export interface DailyUsagePoint {
  date: string;
  used_quota_usd: number;
}

export interface DailyUsageSummary {
  total_used_quota_usd: number;
  today_used_quota_usd: number;
  points: DailyUsagePoint[];
}

export interface ScreenOverview {
  generated_at: string;
  accounts_total: number;
  accounts_enabled: number;
  low_balance_count: number;
  used_groups_count: number;
  items: Array<{
    account_id: number;
    account_name: string;
    account_remark?: string;
    type: UpstreamType;
    base_url: string;
    enabled: boolean;
    last_status?: string;
    last_error?: string;
    last_checked_at?: string | null;
    last_balance_usd?: number | null;
    low_balance_threshold_usd: number;
    used_groups: Array<{
      group_key: string;
      group_name: string;
      rate_multiplier: number;
      token_usage_count: number;
      used_quota_usd?: number | null;
      has_observed_usage?: boolean;
    }>;
  }>;
}

export interface ScreenLayoutItem {
  account_id: number;
  collapsed: boolean;
}

export interface ScreenLayoutSettings {
  items: ScreenLayoutItem[];
}

export interface CustomMenuLink {
  id: string;
  title: string;
  url: string;
  embed_iframe: boolean;
}

export interface CustomMenuLinksSettings {
  items: CustomMenuLink[];
}

export interface GroupToken {
  id: number;
  name: string;
  masked_key: string;
  status: string;
  last_used_at?: string | null;
  used_quota_usd?: number | null;
}

export interface GroupDetail {
  group_key: string;
  group_name: string;
  description: string;
  rate_multiplier: number;
  image_price_1k?: number | null;
  image_price_2k?: number | null;
  image_price_4k?: number | null;
  token_usage_count: number;
  is_used_by_token: boolean;
  platform?: string;
  catalog_source?: "model_plaza" | "groups_api" | string;
  models_count: number;
  models: Array<{
    name: string;
    platform: string;
  }>;
  tokens: GroupToken[];
}

export interface AccountOverview {
  account: Account;
  latest_run?: DetectionRun | null;
  recent_alerts: AlertEvent[];
  latest_groups: GroupDetail[];
}

export interface SMTPSettings {
  enabled: boolean;
  host: string;
  port: number;
  username: string;
  password: string;
  from: string;
  recipients: string[];
  use_tls: boolean;
}

export interface LogCleanupResult {
  log_retention_days: number;
  clean_before: string;
  deleted: {
    balance_snapshots: number;
    group_snapshots: number;
    token_snapshots: number;
    alert_events: number;
    detection_runs: number;
  };
  deleted_total: number;
}

export interface SettingsSnapshot {
  schedule_interval_minutes: number;
  log_retention_days: number;
  smtp: SMTPSettings;
  custom_menu_links: CustomMenuLink[];
  proxy: ProxySettings;
}

export interface ProxySettings {
  host: string;
  port: number;
  username: string;
  password?: string;
  has_password: boolean;
  clear_password?: boolean;
}

export interface SettingsExportBundle {
  version?: string;
  exported_at?: string;
  settings: SettingsSnapshot;
}
