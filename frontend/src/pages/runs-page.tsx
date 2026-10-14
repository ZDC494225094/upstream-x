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
import { formatDateTime, formatDuration, formatRunStatus, formatTriggeredBy } from "@/lib/utils";

export function RunsPage() {
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
        setError(loadError instanceof Error ? loadError.message : "加载检测任务失败");
      }
    })();
  }, [refreshKey]);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-semibold">检测任务</h1>
        <p className="mt-2 text-sm text-muted-foreground">查看全局最近检测任务，重点关注失败、耗时和抓取量。</p>
      </div>

      {error && <ErrorAlert title="加载检测任务失败" message={error} />}

      <Card>
        <CardHeader>
          <div>
            <CardTitle>最近检测任务</CardTitle>
            <CardDescription>当前展示最近 10 次全局检测记录。</CardDescription>
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          {summary?.latest_runs.length ? (
            summary.latest_runs.map((run) => (
              <div key={run.id} className="rounded-md border bg-background p-4">
                <div className="flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <div className="text-sm font-medium">{run.account_name || `账号 #${run.account_id}`}</div>
                      {run.account_type ? <UpstreamTypeBadge type={run.account_type} className="h-6" /> : null}
                    </div>
                    <div className="mt-1 text-xs text-muted-foreground">账号 ID：{run.account_id}</div>
                  </div>
                  <Badge variant={run.status === "success" ? "success" : "destructive"}>{formatRunStatus(run.status)}</Badge>
                </div>
                <div className="mt-2 text-sm text-muted-foreground">
                  检测时间：{formatDateTime(run.completed_at)} ｜ 触发方式：{formatTriggeredBy(run.triggered_by)} ｜ 耗时：{formatDuration(run.duration_millis)}
                </div>
                <div className="mt-1 text-sm text-muted-foreground">
                  分组 {run.groups_count} ｜ 令牌 {run.tokens_count}
                </div>
                {run.error_message && (
                  <Collapsible className="mt-3 rounded-md border border-destructive/20 bg-destructive/5 px-3 py-2">
                    <CollapsibleTrigger className="flex w-full items-center justify-between gap-3 text-left text-sm font-medium text-destructive">
                      <span>查看错误日志</span>
                      <ChevronDown className="h-4 w-4 shrink-0" />
                    </CollapsibleTrigger>
                    <CollapsibleContent className="pt-2">
                      <pre className="overflow-x-auto whitespace-pre-wrap break-words text-xs leading-5 text-destructive">
                        {run.error_message}
                      </pre>
                    </CollapsibleContent>
                  </Collapsible>
                )}
              </div>
            ))
          ) : (
            <div className="text-sm text-muted-foreground">暂无检测记录</div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
