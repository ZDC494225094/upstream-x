import { useEffect, useMemo, useState } from "react";
import { ChevronDown, RefreshCw } from "lucide-react";
import { useParams } from "react-router-dom";

import { BalanceTrendChart, GroupRatioChart } from "@/components/charts";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { ErrorAlert } from "@/components/ui/error-alert";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { UpstreamTypeBadge } from "@/components/upstream-type-badge";
import { usePageRefresh } from "@/hooks/use-page-refresh";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { api } from "@/lib/api";
import type { AccountOverview, DetectionRun, GroupDetail } from "@/lib/types";
import { type ChartGranularity, currency, formatDateTime, formatDuration, formatRangeLabel, formatRunStatus, formatTriggeredBy } from "@/lib/utils";

const ranges = ["3d", "7d", "30d", "90d", "all"];
const chartGranularityOptions: Array<{ label: string; value: ChartGranularity }> = [
  { label: "按时", value: "hour" },
  { label: "按天", value: "day" },
  { label: "按月", value: "month" },
];
type GroupFilter = "all" | "created" | "active";

function groupUsedQuota(group: GroupDetail) {
  let collected = false;
  const total = (group.tokens ?? []).reduce((sum, token) => {
    if (token.used_quota_usd == null || Number.isNaN(token.used_quota_usd)) return sum;
    collected = true;
    return sum + token.used_quota_usd;
  }, 0);
  return collected ? total : null;
}

function groupHasMeasuredUsage(group: GroupDetail) {
  return (groupUsedQuota(group) ?? 0) > 0;
}

function groupHasUsageRecord(group: GroupDetail) {
  return groupHasMeasuredUsage(group) || (group.tokens ?? []).some((token) => Boolean(token.last_used_at));
}

function shouldCollapseAlertMessage(message?: string) {
  if (!message) return false;
  return message.length > 140 || message.includes("\n");
}

export function AccountDetailPage() {
  const { refreshKey } = usePageRefresh();
  const { id } = useParams();
  const accountID = Number(id);
  const [overview, setOverview] = useState<AccountOverview | null>(null);
  const [balanceHistory, setBalanceHistory] = useState<Array<{ time: string; balance_usd: number }>>([]);
  const [groupHistory, setGroupHistory] = useState<
    Array<{ run_id: number; time: string; group_key: string; group_name: string; rate_multiplier: number }>
  >([]);
  const [runs, setRuns] = useState<DetectionRun[]>([]);
  const [range, setRange] = useState("3d");
  const [chartGranularity, setChartGranularity] = useState<ChartGranularity>("hour");
  const [groupFilter, setGroupFilter] = useState<GroupFilter>("all");
  const [error, setError] = useState<string | null>(null);
  const [checking, setChecking] = useState(false);

  async function load() {
    try {
      const [overviewData, balanceData, groupData, runData] = await Promise.all([
        api.accountOverview(accountID),
        api.balanceHistory(accountID, range),
        api.groupHistory(accountID, range),
        api.runLogs(accountID),
      ]);
      setOverview(overviewData);
      setBalanceHistory(balanceData.items ?? []);
      setGroupHistory(groupData.items ?? []);
      setRuns(runData.items ?? []);
      setError(null);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "加载详情失败");
    }
  }

  useEffect(() => {
    if (!accountID) return;
    void load();
  }, [accountID, range, refreshKey]);

  const balanceChartData = useMemo(
    () => balanceHistory
      .map((item) => ({
        rawTime: item.time,
        time: item.time,
        [overview?.account.name ?? "balance"]: item.balance_usd,
      }))
      .sort((left, right) => String(left.rawTime ?? "").localeCompare(String(right.rawTime ?? ""))),
    [balanceHistory, overview?.account.name],
  );

  const latestGroups = useMemo(() => {
    return [...(overview?.latest_groups ?? [])].sort((a, b) => {
      if (a.is_used_by_token !== b.is_used_by_token) {
        return a.is_used_by_token ? -1 : 1;
      }
      if (a.token_usage_count !== b.token_usage_count) {
        return b.token_usage_count - a.token_usage_count;
      }
      return a.group_name.localeCompare(b.group_name, "zh-CN");
    });
  }, [overview?.latest_groups]);
  const visibleGroups = useMemo(() => latestGroups.filter((group) => {
    if (groupFilter === "created") return group.is_used_by_token && group.token_usage_count > 0;
    if (groupFilter === "active") return groupHasMeasuredUsage(group);
    return true;
  }), [groupFilter, latestGroups]);
  const recentAlerts = overview?.recent_alerts ?? [];

  async function handleCheck() {
    setChecking(true);
    try {
      await api.checkAccount(accountID);
      await load();
    } catch (checkError) {
      setError(checkError instanceof Error ? checkError.message : "检测失败");
    } finally {
      setChecking(false);
    }
  }

  if (!accountID) {
    return <div className="text-sm text-destructive">无效账号 ID</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="text-3xl font-semibold">{overview?.account.name ?? "账号详情"}</h1>
          <div className="mt-2 flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
            <span>{overview?.account.base_url}</span>
            <UpstreamTypeBadge type={overview?.account.type} className="h-6" />
          </div>
          {overview?.account.remark ? <p className="mt-3 max-w-3xl text-sm text-muted-foreground">{overview.account.remark}</p> : null}
        </div>
        <div className="flex gap-2">
          {ranges.map((item) => (
            <Button key={item} variant={range === item ? "default" : "outline"} size="sm" onClick={() => setRange(item)}>
              {formatRangeLabel(item)}
            </Button>
          ))}
          <Button variant="secondary" onClick={handleCheck} disabled={checking}>
            <RefreshCw className={`mr-2 h-4 w-4 ${checking ? "animate-spin" : ""}`} />
            {checking ? "检测中..." : "立即检测"}
          </Button>
        </div>
      </div>

      {error && <ErrorAlert title="加载账号详情失败" message={error} />}

      <div className="grid gap-4 xl:grid-cols-3">
        <Card>
          <CardHeader>
            <div>
              <CardDescription>当前余额</CardDescription>
              <CardTitle className="text-3xl">{currency(overview?.account.last_balance_usd)}</CardTitle>
            </div>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <div>
              <CardDescription>低余额阈值</CardDescription>
              <CardTitle className="text-3xl">{currency(overview?.account.low_balance_threshold_usd)}</CardTitle>
            </div>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <div>
              <CardDescription>最近检测</CardDescription>
              <CardTitle className="break-words text-3xl leading-tight">
                {formatDateTime(overview?.account.last_checked_at)}
              </CardTitle>
            </div>
          </CardHeader>
        </Card>
      </div>

      <Tabs defaultValue="groups">
        <TabsList>
          <TabsTrigger value="groups">分组</TabsTrigger>
          <TabsTrigger value="charts">图表</TabsTrigger>
          <TabsTrigger value="logs">日志</TabsTrigger>
        </TabsList>

        <TabsContent value="groups">
          <Card>
            <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <CardTitle>分组与令牌占用</CardTitle>
                <CardDescription>
                  共 {latestGroups.length} 个分组{visibleGroups.length !== latestGroups.length ? `，当前显示 ${visibleGroups.length} 个` : ""}。
                </CardDescription>
              </div>
              <Select value={groupFilter} onValueChange={(value: GroupFilter) => setGroupFilter(value)}>
                <SelectTrigger className="w-full sm:w-40" aria-label="筛选分组">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部分组</SelectItem>
                  <SelectItem value="created">已创建令牌</SelectItem>
                  <SelectItem value="active">已产生用量</SelectItem>
                </SelectContent>
              </Select>
            </CardHeader>
            <CardContent>
              {visibleGroups.length ? (
                <div className="columns-1 [column-gap:1rem] xl:columns-2">
                  {visibleGroups.map((group) => {
                    const tokens = group.tokens ?? [];
                    const usedQuota = groupUsedQuota(group);
                    const hasUsage = groupHasUsageRecord(group);
                    const priceItems = [
                      group.image_price_1k != null ? { label: "1k", value: group.image_price_1k } : null,
                      group.image_price_2k != null ? { label: "2k", value: group.image_price_2k } : null,
                      group.image_price_4k != null ? { label: "4k", value: group.image_price_4k } : null,
                    ].filter((item): item is { label: string; value: number } => item !== null);
                    return (
                      <div key={group.group_key} className="mb-4 break-inside-avoid rounded-xl border bg-card p-4">
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0 flex-1 space-y-2">
                            <div className="flex flex-wrap items-center gap-2">
                              <div className="truncate text-base font-semibold">{group.group_name}</div>
                              {group.catalog_source === "model_plaza" ? (
                                <Badge variant="default">模型广场 {group.models_count}</Badge>
                              ) : null}
                              {group.is_used_by_token ? <Badge variant="outline">已创建令牌 {group.token_usage_count}</Badge> : null}
                              {hasUsage ? <Badge variant="success">已使用</Badge> : null}
                            </div>
                            {group.description ? <div className="text-xs leading-5 text-muted-foreground">{group.description}</div> : null}
                          </div>
                          <div className="shrink-0 rounded-lg border bg-muted/30 px-3 py-2 text-right">
                            <div className="text-[11px] text-muted-foreground">倍率</div>
                            <div className="text-lg font-semibold leading-none">{group.rate_multiplier}</div>
                          </div>
                        </div>

                        <div className="mt-4 grid gap-2 sm:grid-cols-3">
                          <div className="rounded-lg bg-muted/35 px-3 py-2">
                            <div className="text-[11px] text-muted-foreground">分组标识</div>
                            <div className="mt-1 break-all text-sm font-medium">{group.group_key}</div>
                          </div>
                          <div className="rounded-lg bg-muted/35 px-3 py-2">
                            <div className="text-[11px] text-muted-foreground">令牌占用</div>
                            <div className="mt-1 text-sm font-medium">{tokens.length} 个令牌</div>
                          </div>
                          <div className="rounded-lg bg-muted/35 px-3 py-2">
                            <div className="text-[11px] text-muted-foreground">累计用量</div>
                            <div className="mt-1 text-sm font-medium">
                              {usedQuota != null ? currency(usedQuota) : hasUsage ? "已有记录" : "未采集"}
                            </div>
                          </div>
                        </div>

                        {priceItems.length > 0 && (
                          <div className="mt-3 flex flex-wrap gap-2">
                            {priceItems.map((item) => (
                              <div key={item.label} className="rounded-md border bg-muted/20 px-3 py-2 text-xs">
                                {item.label} 图价：{currency(item.value)}
                              </div>
                            ))}
                          </div>
                        )}

                        {(group.models ?? []).length > 0 ? (
                          <Collapsible className="mt-4 border-t pt-4">
                            <div className="flex items-center justify-between gap-3">
                              <div>
                                <div className="text-sm font-medium">可用模型</div>
                                <div className="mt-1 text-xs text-muted-foreground">
                                  来自模型广场，不需要为每个分组额外创建密钥。
                                </div>
                              </div>
                              <CollapsibleTrigger asChild>
                                <Button variant="outline" size="sm">
                                  查看 {group.models_count} 个
                                  <ChevronDown className="ml-2 h-4 w-4" />
                                </Button>
                              </CollapsibleTrigger>
                            </div>
                            <CollapsibleContent className="pt-3">
                              <ScrollArea className="h-52 rounded-md border">
                                <div className="flex flex-wrap gap-2 p-3">
                                  {group.models.map((model) => (
                                    <Badge key={`${model.platform}:${model.name}`} variant="outline" className="h-auto min-h-6 py-1">
                                      {model.name}
                                      {model.platform && model.platform !== group.platform ? (
                                        <span className="ml-1 text-muted-foreground">· {model.platform}</span>
                                      ) : null}
                                    </Badge>
                                  ))}
                                </div>
                              </ScrollArea>
                            </CollapsibleContent>
                          </Collapsible>
                        ) : null}

                        {tokens.length > 0 && (
                          <div className="mt-4 space-y-2 border-t pt-4">
                            {tokens.map((token) => (
                              <div key={token.id} className="rounded-md border bg-background px-3 py-2 text-xs">
                                <div className="flex items-center justify-between gap-3">
                                  <div className="min-w-0">
                                    <div className="truncate font-medium">{token.name}</div>
                                    <div className="mt-1 truncate text-muted-foreground">{token.masked_key}</div>
                                  </div>
                                  <Badge variant="outline" className="h-5 px-2 text-[11px]">
                                    {token.status}
                                  </Badge>
                                </div>
                                <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-muted-foreground">
                                  <span>最近使用 {formatDateTime(token.last_used_at)}</span>
                                  <span>
                                    {token.used_quota_usd != null ? `已用 ${currency(token.used_quota_usd)}` : "用量未采集"}
                                  </span>
                                </div>
                              </div>
                            ))}
                          </div>
                        )}

                        {!tokens.length && (
                          <div className="mt-4 rounded-lg border border-dashed px-3 py-2 text-xs text-muted-foreground">
                            当前没有令牌使用这个分组
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              ) : (
                <div className="text-sm text-muted-foreground">
                  {latestGroups.length ? "暂无符合筛选条件的分组" : "暂无分组数据"}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="charts">
          <div className="grid gap-6">
            <Card>
              <CardHeader>
                <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                  <div>
                    <CardTitle>余额趋势</CardTitle>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {chartGranularityOptions.map((item) => (
                      <Button
                        key={item.value}
                        variant={chartGranularity === item.value ? "default" : "outline"}
                        size="sm"
                        onClick={() => setChartGranularity(item.value)}
                      >
                        {item.label}
                      </Button>
                    ))}
                  </div>
                </div>
              </CardHeader>
              <CardContent>{balanceChartData.length ? <BalanceTrendChart data={balanceChartData} granularity={chartGranularity} /> : <div className="text-sm text-muted-foreground">暂无余额历史</div>}</CardContent>
            </Card>
            <Card>
              <CardHeader>
                <div>
                  <CardTitle>分组倍率历史</CardTitle>
                  <CardDescription>展示最近检测区间内的倍率变化。</CardDescription>
                </div>
              </CardHeader>
              <CardContent>{groupHistory.length ? <GroupRatioChart data={groupHistory} granularity={chartGranularity} /> : <div className="text-sm text-muted-foreground">暂无倍率历史</div>}</CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="logs">
          <div className="grid gap-6 xl:grid-cols-2">
            <Card>
              <CardHeader>
                <div>
                  <CardTitle>检测日志</CardTitle>
                  <CardDescription>最近 50 次任务执行结果。</CardDescription>
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                {runs.length ? (
                  runs.map((run) => (
                    <div key={run.id} className="rounded-md border bg-background p-4">
                      <div className="flex items-center justify-between gap-3">
                        <Badge variant={run.status === "success" ? "success" : "destructive"}>{formatRunStatus(run.status)}</Badge>
                        <div className="text-xs text-muted-foreground">{formatDateTime(run.completed_at)}</div>
                      </div>
                      <div className="mt-2 text-sm text-muted-foreground">
                        触发方式 {formatTriggeredBy(run.triggered_by)} ｜ 分组 {run.groups_count} ｜ 令牌 {run.tokens_count} ｜ 耗时 {formatDuration(run.duration_millis)}
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
                  <div className="text-sm text-muted-foreground">暂无检测日志</div>
                )}
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <div>
                  <CardTitle>变更与告警</CardTitle>
                  <CardDescription>倍率、价格、余额与故障状态的变化记录。</CardDescription>
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                {recentAlerts.length ? (
                  recentAlerts.map((alert) => (
                    <div key={alert.id} className="rounded-md border bg-background p-4">
                      <div className="flex items-center justify-between gap-3">
                        <div className="font-medium">{alert.title}</div>
                        <div className="text-xs text-muted-foreground">{formatDateTime(alert.created_at)}</div>
                      </div>
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
                        <div className="mt-2 text-xs text-muted-foreground">
                          {alert.prev_value ? `旧值：${alert.prev_value}` : ""} {alert.current_value ? `新值：${alert.current_value}` : ""}
                        </div>
                      )}
                    </div>
                  ))
                ) : (
                  <div className="text-sm text-muted-foreground">暂无变更日志</div>
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
