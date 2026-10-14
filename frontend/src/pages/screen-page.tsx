import { ChevronDown, Clock3, Eye, ExternalLink, GripVertical, RefreshCw, Wallet } from "lucide-react";
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";

import { UpstreamTypeBadge } from "@/components/upstream-type-badge";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { ErrorAlert } from "@/components/ui/error-alert";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { usePageRefresh } from "@/hooks/use-page-refresh";
import { useToast } from "@/hooks/use-toast";
import { api } from "@/lib/api";
import type { ScreenLayoutItem, ScreenOverview } from "@/lib/types";
import { currency, formatDateTime } from "@/lib/utils";

function formatAccountStatus(status?: string | null) {
  if (status === "success") return "成功";
  if (status === "failed") return "失败";
  return "未检测";
}

function getStatusVariant(status?: string | null) {
  if (status === "success") return "success" as const;
  if (status === "failed") return "destructive" as const;
  return "default" as const;
}

type ScreenGroupView = "all" | "active";
type ScreenGroup = ScreenOverview["items"][number]["used_groups"][number];

function hasMeasuredGroupUsage(group: ScreenGroup) {
  return (group.used_quota_usd ?? 0) > 0;
}

function visibleScreenGroups(groups: ScreenGroup[], view: ScreenGroupView) {
  return view === "active" ? groups.filter(hasMeasuredGroupUsage) : groups;
}

function mergeLayoutItems(
  sourceItems: NonNullable<ScreenOverview["items"]>,
  currentLayout: ScreenLayoutItem[],
) {
  const sourceIDs = new Set(sourceItems.map((item) => item.account_id));
  const next = currentLayout.filter((item) => sourceIDs.has(item.account_id));
  const seen = new Set(next.map((item) => item.account_id));

  for (const item of sourceItems) {
    if (!seen.has(item.account_id)) {
      next.push({ account_id: item.account_id, collapsed: false });
    }
  }

  return next;
}

function getScreenColumnCount(width: number) {
  if (width >= 1536) return 3;
  if (width >= 1280) return 2;
  return 1;
}

export function ScreenPage() {
  const { refreshKey } = usePageRefresh();
  const { toast } = useToast();
  const [data, setData] = useState<ScreenOverview | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [checkingId, setCheckingId] = useState<number | null>(null);
  const [groupView, setGroupView] = useState<ScreenGroupView>("all");
  const [layoutItems, setLayoutItems] = useState<ScreenLayoutItem[]>([]);
  const [draggingId, setDraggingId] = useState<number | null>(null);
  const [columnCount, setColumnCount] = useState(() =>
    typeof window === "undefined" ? 1 : getScreenColumnCount(window.innerWidth),
  );
  const [error, setError] = useState<string | null>(null);
  const cardRefs = useRef(new Map<number, HTMLDivElement>());
  const pendingAnchorRef = useRef<{ accountID: number; top: number } | null>(null);

  async function saveLayout(items: ScreenLayoutItem[]) {
    const normalized = items.map((item) => ({ account_id: item.account_id, collapsed: item.collapsed }));
    await api.updateScreenLayout({ items: normalized });
  }

  async function load() {
    setLoading(true);
    try {
      const [result, layout] = await Promise.all([
        api.dashboardScreen(),
        api.getScreenLayout(),
      ]);
      setData(result);
      setLayoutItems(mergeLayoutItems(result.items ?? [], layout.items ?? []));
      setError(null);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "加载大屏失败");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [refreshKey]);

  useEffect(() => {
    function handleResize() {
      setColumnCount(getScreenColumnCount(window.innerWidth));
    }

    handleResize();
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  async function handleRefresh() {
    setRefreshing(true);
    setError(null);
    try {
      const result = await api.refreshDashboardData();
      setData(result);
      toast({ title: "已完成全账号刷新检测" });
    } catch (refreshError) {
      setError(refreshError instanceof Error ? refreshError.message : "刷新数据失败");
    } finally {
      setRefreshing(false);
    }
  }

  async function handleCheck(accountID: number) {
    setCheckingId(accountID);
    setError(null);
    try {
      await api.checkAccount(accountID);
      const result = await api.dashboardScreen();
      setData(result);
      toast({ title: "已完成账号检测" });
    } catch (checkError) {
      setError(checkError instanceof Error ? checkError.message : "账号检测失败");
    } finally {
      setCheckingId(null);
    }
  }

  const orderedItems = useMemo(() => {
    const sourceItems = data?.items ?? [];
    if (!sourceItems.length) return [];

    const collapsedMap = new Map<number, boolean>();
    const orderMap = new Map<number, number>();
    layoutItems.forEach((item, index) => {
      collapsedMap.set(item.account_id, item.collapsed);
      orderMap.set(item.account_id, index);
    });

    return [...sourceItems]
      .sort((left, right) => {
        const leftIndex = orderMap.get(left.account_id);
        const rightIndex = orderMap.get(right.account_id);
        if (leftIndex === undefined && rightIndex === undefined) return 0;
        if (leftIndex === undefined) return 1;
        if (rightIndex === undefined) return -1;
        return leftIndex - rightIndex;
      })
      .map((item) => ({
        ...item,
        collapsed: collapsedMap.get(item.account_id) ?? false,
      }));
  }, [data, layoutItems]);

  const masonryColumns = useMemo(() => {
    const count = Math.max(1, Math.min(columnCount, orderedItems.length || 1));
    const columns = Array.from({ length: count }, () => [] as typeof orderedItems);

    orderedItems.forEach((item, index) => {
      columns[index % count].push(item);
    });

    return columns;
  }, [columnCount, orderedItems]);

  useEffect(() => {
    if (!data?.items.length) return;

    setLayoutItems((current) => {
      const next = mergeLayoutItems(data.items, current);
      const same =
        next.length === current.length &&
        next.every((item, index) => item.account_id === current[index]?.account_id && item.collapsed === current[index]?.collapsed);
      return same ? current : next;
    });
  }, [data]);

  useLayoutEffect(() => {
    const anchor = pendingAnchorRef.current;
    if (!anchor) return;

    const nextElement = cardRefs.current.get(anchor.accountID);
    pendingAnchorRef.current = null;
    if (!nextElement) return;

    const delta = nextElement.getBoundingClientRect().top - anchor.top;
    if (Math.abs(delta) > 1) {
      window.scrollTo({
        left: window.scrollX,
        top: window.scrollY + delta,
        behavior: "auto",
      });
    }
  }, [layoutItems, columnCount]);

  function rememberCardPosition(accountID: number) {
    const element = cardRefs.current.get(accountID);
    if (!element) return;

    pendingAnchorRef.current = {
      accountID,
      top: element.getBoundingClientRect().top,
    };
  }

  async function updateCollapsed(accountID: number, collapsed: boolean) {
    const nextItems = layoutItems.map((item) =>
      item.account_id === accountID ? { ...item, collapsed } : item,
    );
    setLayoutItems(nextItems);
    try {
      await saveLayout(nextItems);
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "保存大屏布局失败");
    }
  }

  async function moveCard(draggedID: number, targetID: number) {
    if (draggedID === targetID) return;
    const current = [...layoutItems];
    const draggedIndex = current.findIndex((item) => item.account_id === draggedID);
    const targetIndex = current.findIndex((item) => item.account_id === targetID);
    if (draggedIndex < 0 || targetIndex < 0) return;

    const [dragged] = current.splice(draggedIndex, 1);
    current.splice(targetIndex, 0, dragged);
    setLayoutItems(current);
    try {
      await saveLayout(current);
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "保存大屏布局失败");
    }
  }

  if (loading) {
    return <div className="text-sm text-muted-foreground">加载大屏中...</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <h1 className="text-3xl font-semibold">账号大屏</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            集中展示账号余额、令牌分组、实际用量和对应分组倍率。
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <Select value={groupView} onValueChange={(value: ScreenGroupView) => setGroupView(value)}>
            <SelectTrigger className="w-40" aria-label="筛选令牌分组">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部令牌分组</SelectItem>
              <SelectItem value="active">已产生用量</SelectItem>
            </SelectContent>
          </Select>
          <div className="text-xs text-muted-foreground">
            最近生成：{formatDateTime(data?.generated_at)}
          </div>
          <Button onClick={() => void handleRefresh()} disabled={refreshing}>
            <RefreshCw className={`mr-2 h-4 w-4 ${refreshing ? "animate-spin" : ""}`} />
            {refreshing ? "刷新中..." : "立即刷新数据"}
          </Button>
        </div>
      </div>

      {error && <ErrorAlert title="加载大屏失败" message={error} />}

      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Card>
          <CardHeader className="mb-0">
            <div>
              <CardDescription>账号总数</CardDescription>
              <CardTitle className="mt-2 text-3xl">{data?.accounts_total ?? "--"}</CardTitle>
            </div>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="mb-0">
            <div>
              <CardDescription>启用账号</CardDescription>
              <CardTitle className="mt-2 text-3xl">{data?.accounts_enabled ?? "--"}</CardTitle>
            </div>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="mb-0">
            <div>
              <CardDescription>低余额账号</CardDescription>
              <CardTitle className="mt-2 text-3xl">{data?.low_balance_count ?? "--"}</CardTitle>
            </div>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="mb-0">
            <div>
              <CardDescription>已占用分组</CardDescription>
              <CardTitle className="mt-2 text-3xl">{data?.used_groups_count ?? "--"}</CardTitle>
            </div>
          </CardHeader>
        </Card>
      </div>

      <div
        className="grid items-start gap-5 [overflow-anchor:none]"
        style={{ gridTemplateColumns: `repeat(${masonryColumns.length}, minmax(0, 1fr))` }}
      >
        {masonryColumns.map((column, columnIndex) => (
          <div key={`screen-column-${columnIndex}`} className="flex min-w-0 flex-col gap-5">
            {column.map((item) => (
              <div
                key={item.account_id}
                ref={(node) => {
                  if (node) {
                    cardRefs.current.set(item.account_id, node);
                    return;
                  }
                  cardRefs.current.delete(item.account_id);
                }}
              >
                <Card
                  className={`relative overflow-hidden border bg-card shadow-sm ${draggingId === item.account_id ? "opacity-70" : ""}`}
                  draggable
                  onDragStart={() => setDraggingId(item.account_id)}
                  onDragEnd={() => setDraggingId(null)}
                  onDragOver={(event) => event.preventDefault()}
                  onDrop={() => {
                    if (draggingId) {
                      void moveCard(draggingId, item.account_id);
                    }
                    setDraggingId(null);
                  }}
                >
                  <CardHeader className="mb-0 pb-4">
                    <div className="min-w-0 space-y-3">
                      <div className="flex items-start gap-3">
                        <div className="mt-1 shrink-0 cursor-grab text-muted-foreground/70 active:cursor-grabbing">
                          <GripVertical className="h-4 w-4" />
                        </div>
                        <div className="min-w-0 flex-1 space-y-2.5">
                          <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-2 gap-y-2">
                            <CardTitle className="min-w-0 text-base font-semibold leading-5 tracking-tight sm:text-lg sm:leading-6">
                              {item.account_name}
                            </CardTitle>
                            <div className="flex shrink-0 flex-wrap items-center justify-end gap-1.5">
                              <UpstreamTypeBadge
                                type={item.type}
                                className="h-5 gap-1 pl-1.5 pr-2 text-[11px] sm:h-6 sm:gap-1.5 sm:pl-1.5 sm:pr-2.5 sm:text-xs"
                              />
                              <Badge
                                className="h-5 px-2 text-[11px] sm:h-6 sm:px-2.5 sm:text-xs"
                                variant={getStatusVariant(item.last_status)}
                              >
                                {formatAccountStatus(item.last_status)}
                              </Badge>
                              {!item.enabled && (
                                <Badge className="h-5 px-2 text-[11px] sm:h-6 sm:px-2.5 sm:text-xs" variant="default">
                                  已停用
                                </Badge>
                              )}
                            </div>
                          </div>

                          <div className="space-y-1.5">
                            {item.account_remark ? (
                              <div className="min-w-0 break-words text-sm leading-5 text-foreground/80">
                                {item.account_remark}
                              </div>
                            ) : null}
                            <CardDescription className="block min-w-0 break-all font-mono text-[11px] leading-4 text-muted-foreground">
                              {item.base_url}
                            </CardDescription>
                          </div>
                        </div>
                      </div>
                    </div>
                  </CardHeader>
                <Collapsible
                  open={!item.collapsed}
                  onOpenChange={(open) => {
                    rememberCardPosition(item.account_id);
                    void updateCollapsed(item.account_id, !open);
                  }}
                >
                  <CardContent className="space-y-5 pt-4">
                    <div className="rounded-lg border bg-muted/10 px-3 py-3">
                      <div className="grid gap-3 sm:grid-cols-2 sm:gap-0">
                        <div className="space-y-1 sm:border-r sm:pr-3">
                          <div className="text-[11px] text-muted-foreground">当前余额</div>
                          <div className="flex items-center gap-2">
                            <Wallet className="h-4 w-4 text-muted-foreground" />
                            <span className="text-base font-semibold leading-6 sm:text-lg">{currency(item.last_balance_usd)}</span>
                          </div>
                        </div>
                        <div className="space-y-1 sm:pl-3">
                          <div className="text-[11px] text-muted-foreground">最近检测</div>
                          <div className="flex items-center gap-2">
                            <Clock3 className="h-4 w-4 text-muted-foreground" />
                            <span className="text-sm font-medium leading-5 sm:text-base">{formatDateTime(item.last_checked_at)}</span>
                          </div>
                        </div>
                      </div>
                    </div>

                    {item.last_status === "failed" && item.last_error ? (
                      <Collapsible className="rounded-md border border-destructive/20 bg-destructive/5 px-3 py-2">
                        <CollapsibleTrigger className="flex w-full items-center justify-between gap-3 text-left text-sm font-medium text-destructive">
                          <span>查看错误日志</span>
                          <ChevronDown className="h-4 w-4 shrink-0" />
                        </CollapsibleTrigger>
                        <CollapsibleContent className="pt-2">
                          <pre className="overflow-x-auto whitespace-pre-wrap break-words text-xs leading-5 text-destructive">
                            {item.last_error}
                          </pre>
                        </CollapsibleContent>
                      </Collapsible>
                    ) : null}

                    <div className="flex justify-end">
                      <CollapsibleTrigger className="flex items-center gap-1 text-xs font-medium text-muted-foreground">
                        <span>{item.collapsed ? "展开卡片" : "折叠卡片"}</span>
                        <ChevronDown className={`h-4 w-4 transition-transform ${item.collapsed ? "-rotate-90" : "rotate-0"}`} />
                      </CollapsibleTrigger>
                    </div>
                    <CollapsibleContent className="space-y-5">
                      <div className="space-y-3">
                          <div className="flex items-center justify-between gap-3">
                            <div className="text-sm font-medium">令牌已占用分组</div>
                            <div className="flex flex-wrap justify-end gap-x-3 gap-y-1 text-xs text-muted-foreground">
                              <span>{item.used_groups.length} 个分组</span>
                              <span>{item.used_groups.filter(hasMeasuredGroupUsage).length} 个有用量</span>
                              <span>阈值 {currency(item.low_balance_threshold_usd)}</span>
                            </div>
                          </div>
                          {visibleScreenGroups(item.used_groups, groupView).length ? (
                            <div className="grid gap-2">
                              {visibleScreenGroups(item.used_groups, groupView).map((group) => (
                                <div
                                  key={`${item.account_id}-${group.group_key}`}
                                  className="flex flex-col gap-3 rounded-lg border bg-background px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
                                >
                                  <div className="min-w-0 flex-1">
                                    <div className="break-all text-sm font-medium leading-5">{group.group_name}</div>
                                    <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
                                      <span className="break-all">标识 {group.group_key}</span>
                                      <span>令牌 {group.token_usage_count}</span>
                                    </div>
                                  </div>
                                  <div className="shrink-0 self-start text-left sm:text-right">
                                    <div className="text-[11px] text-muted-foreground">倍率</div>
                                    <div className="text-lg font-semibold">{group.rate_multiplier}</div>
                                    {hasMeasuredGroupUsage(group) ? (
                                      <div className="mt-1 text-xs font-medium text-emerald-700 dark:text-emerald-300">
                                        已用 {currency(group.used_quota_usd)}
                                      </div>
                                    ) : group.has_observed_usage ? (
                                      <div className="mt-1 text-xs text-muted-foreground">已有使用记录</div>
                                    ) : (
                                      <div className="mt-1 text-xs text-muted-foreground">暂无用量</div>
                                    )}
                                  </div>
                                </div>
                              ))}
                            </div>
                          ) : (
                            <div className="rounded-lg border border-dashed px-4 py-6 text-sm text-muted-foreground">
                              {groupView === "active" ? "暂无已产生用量的分组" : "当前没有令牌占用分组"}
                            </div>
                          )}
                        </div>

                        <div className="flex flex-wrap justify-end gap-2 border-t pt-4">
                          <Button
                            size="sm"
                            variant="secondary"
                            onClick={() => void handleCheck(item.account_id)}
                            disabled={checkingId === item.account_id}
                          >
                            <RefreshCw className={`mr-2 h-4 w-4 ${checkingId === item.account_id ? "animate-spin" : ""}`} />
                            {checkingId === item.account_id ? "检测中..." : "立即检测"}
                          </Button>
                        <Button size="sm" variant="outline" asChild>
                          <Link to={`/accounts/${item.account_id}`}>
                            <Eye className="mr-2 h-4 w-4" />
                            查看详情
                          </Link>
                        </Button>
                        <Button size="sm" variant="outline" asChild>
                          <a href={item.base_url} target="_blank" rel="noreferrer">
                            <ExternalLink className="mr-2 h-4 w-4" />
                            访问网站
                          </a>
                        </Button>
                      </div>
                      </CollapsibleContent>
                    </CardContent>
                  </Collapsible>
                </Card>
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}
