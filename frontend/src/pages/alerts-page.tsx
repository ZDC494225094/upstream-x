import { ChevronDown } from "lucide-react";
import { useEffect, useState } from "react";

import { UpstreamTypeBadge } from "@/components/upstream-type-badge";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { ErrorAlert } from "@/components/ui/error-alert";
import { usePageRefresh } from "@/hooks/use-page-refresh";
import { api } from "@/lib/api";
import type { DashboardSummary } from "@/lib/types";
import { formatDateTime } from "@/lib/utils";

function formatSeverity(severity: string) {
  if (severity === "error") return { label: "异常", variant: "destructive" as const };
  if (severity === "warning") return { label: "告警", variant: "warning" as const };
  return { label: "信息", variant: "default" as const };
}

function formatAlertType(type?: string) {
  if (type === "low_balance") return "低余额";
  if (type === "balance_recovery") return "余额恢复";
  if (type === "account_error") return "检测异常";
  if (type === "account_recovery") return "检测恢复";
  if (type === "group_changed") return "分组变更";
  return type || "事件";
}

function shouldCollapseAlertMessage(message?: string) {
  if (!message) return false;
  return message.length > 140 || message.includes("\n");
}

export function AlertsPage() {
  const { refreshKey } = usePageRefresh();
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    void (async () => {
      try {
        const data = await api.dashboard("3d");
        setSummary(data);
        setError(null);
      } catch (loadError) {
        setError(loadError instanceof Error ? loadError.message : "加载告警与变更失败");
      }
    })();
  }, [refreshKey]);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-semibold">告警与变更</h1>
        <p className="mt-2 text-sm text-muted-foreground">集中查看最近的余额告警、异常恢复、分组倍率和价格变化记录。</p>
      </div>

      {error && <ErrorAlert title="加载告警与变更失败" message={error} />}

      <Card>
        <CardHeader>
          <div>
            <CardTitle>最近告警与变更</CardTitle>
            <CardDescription>当前展示最近 20 条全局告警和变化事件。</CardDescription>
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          {summary?.recent_alerts.length ? (
            summary.recent_alerts.map((alert) => {
              const severity = formatSeverity(alert.severity);
              return (
                <div key={alert.id} className="rounded-md border bg-background p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <div className="text-sm font-medium">{alert.account_name || `账号 #${alert.account_id}`}</div>
                        {alert.account_type ? <UpstreamTypeBadge type={alert.account_type} className="h-6" /> : null}
                        <Badge variant={severity.variant}>{severity.label}</Badge>
                        <Badge variant="outline">{formatAlertType(alert.type)}</Badge>
                      </div>
                      <div className="mt-1 text-xs text-muted-foreground">账号 ID：{alert.account_id}</div>
                    </div>
                    <div className="shrink-0 text-xs text-muted-foreground">{formatDateTime(alert.created_at)}</div>
                  </div>
                  <div className="mt-3 text-sm font-medium">{alert.title}</div>
                  {shouldCollapseAlertMessage(alert.message) ? (
                    <Collapsible className="mt-2 rounded-md border bg-muted/20 px-3 py-2">
                      <div className="text-sm text-muted-foreground line-clamp-2">{alert.message}</div>
                      <CollapsibleTrigger className="mt-2 flex items-center gap-1 text-xs font-medium text-foreground">
                        <span>展开详情</span>
                        <ChevronDown className="h-3.5 w-3.5" />
                      </CollapsibleTrigger>
                      <CollapsibleContent className="pt-2">
                        <pre className="whitespace-pre-wrap break-words text-xs leading-5 text-muted-foreground">
                          {alert.message}
                        </pre>
                      </CollapsibleContent>
                    </Collapsible>
                  ) : (
                    <div className="mt-2 text-sm text-muted-foreground">{alert.message}</div>
                  )}
                  {(alert.prev_value || alert.current_value) && (
                    <div className="mt-3 grid gap-2 sm:grid-cols-2">
                      {alert.prev_value ? (
                        <div className="rounded-lg border bg-muted/30 px-3 py-2 text-xs">
                          <div className="text-muted-foreground">旧值</div>
                          <div className="mt-1 break-all font-medium text-foreground">{alert.prev_value}</div>
                        </div>
                      ) : null}
                      {alert.current_value ? (
                        <div className="rounded-lg border bg-muted/30 px-3 py-2 text-xs">
                          <div className="text-muted-foreground">新值</div>
                          <div className="mt-1 break-all font-medium text-foreground">{alert.current_value}</div>
                        </div>
                      ) : null}
                    </div>
                  )}
                  {alert.group_key ? (
                    <div className="mt-3 text-xs text-muted-foreground">
                      分组标识：<span className="break-all">{alert.group_key}</span>
                    </div>
                  ) : null}
                </div>
              );
            })
          ) : (
            <div className="text-sm text-muted-foreground">暂无告警日志</div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
