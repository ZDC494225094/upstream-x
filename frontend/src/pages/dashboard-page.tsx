import { useEffect, useMemo, useState } from "react";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { Activity, CalendarDays, CircleDollarSign, Layers3, ShieldAlert, WalletCards } from "lucide-react";

import { BalanceTrendChart } from "@/components/charts";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart";
import { ErrorAlert } from "@/components/ui/error-alert";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { usePageRefresh } from "@/hooks/use-page-refresh";
import { api } from "@/lib/api";
import type { DailyUsageSummary, DashboardSummary } from "@/lib/types";
import { currency, type ChartGranularity, formatRangeLabel } from "@/lib/utils";

const ranges = ["3d", "7d", "30d", "90d", "all"];
const chartGranularityOptions: Array<{ label: string; value: ChartGranularity }> = [
  { label: "按时", value: "hour" },
  { label: "按天", value: "day" },
  { label: "按月", value: "month" },
];

const dailyUsageChartConfig = {
  used_quota_usd: {
    label: "额度消耗",
    color: "hsl(160 84% 30%)",
  },
} satisfies ChartConfig;

function formatDay(value: string, includeYear = false) {
  const date = new Date(`${value}T00:00:00`);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("zh-CN", {
    year: includeYear ? "numeric" : undefined,
    month: "2-digit",
    day: "2-digit",
  }).format(date);
}

export function DashboardPage() {
  const { refreshKey } = usePageRefresh();
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [dailyUsage, setDailyUsage] = useState<DailyUsageSummary | null>(null);
  const [range, setRange] = useState("30d");
  const [chartGranularity, setChartGranularity] = useState<ChartGranularity>("day");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    setLoading(true);
    Promise.all([api.dashboard(range), api.dailyUsage(range)])
      .then(([dashboardData, usageData]) => {
        if (!active) return;
        setSummary(dashboardData);
        setDailyUsage(usageData);
        setError(null);
      })
      .catch((loadError: unknown) => {
        if (!active) return;
        setError(loadError instanceof Error ? loadError.message : "加载失败");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [range, refreshKey]);

  const balanceChartData = useMemo(() => {
    if (!summary) return [];
    const rows: Array<Record<string, string | number>> = [];
    for (const account of summary.account_balances) {
      const accountLabel = `${account.account_name}（${account.account_id}）`;
      for (const point of account.points) {
        rows.push({
          rawTime: point.time,
          time: point.time,
          [accountLabel]: point.balance_usd,
        });
      }
    }
    return rows.sort((left, right) => String(left.rawTime ?? "").localeCompare(String(right.rawTime ?? "")));
  }, [summary]);

  const usagePoints = dailyUsage?.points ?? [];
  const averageDailyUsage = usagePoints.length ? (dailyUsage?.total_used_quota_usd ?? 0) / usagePoints.length : 0;
  const activeUsageDays = usagePoints.filter((point) => point.used_quota_usd > 0).length;
  const hasUsageData = activeUsageDays > 0;
  const recentUsagePoints = [...usagePoints].reverse().slice(0, 10);

  const metricValue = (value: React.ReactNode) =>
    loading ? <Skeleton className="h-9 w-24" /> : <div className="text-3xl font-semibold tracking-normal">{value}</div>;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h1 className="text-2xl font-semibold">总览</h1>
          <p className="mt-1 text-sm text-muted-foreground">汇总所有上游的每日额度消耗、账号状态与余额趋势。</p>
        </div>
        <Tabs value={range} onValueChange={setRange}>
          <TabsList className="grid w-full grid-cols-5 lg:w-auto">
            {ranges.map((item) => (
              <TabsTrigger key={item} value={item} className="px-3">
                {formatRangeLabel(item)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      </div>

      {error ? <ErrorAlert title="加载总览失败" message={error} /> : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <Card className="min-w-0">
          <CardHeader className="mb-3">
            <div>
              <CardDescription>今日消耗</CardDescription>
              <CardTitle className="sr-only">今日消耗额度</CardTitle>
            </div>
            <CircleDollarSign className="h-5 w-5 text-emerald-600" />
          </CardHeader>
          <CardContent>
            {metricValue(currency(dailyUsage?.today_used_quota_usd))}
            <p className="mt-2 text-xs text-muted-foreground">新版优先使用上游每日趋势，旧版按现有密钥快照补算</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="mb-3">
            <div>
              <CardDescription>{formatRangeLabel(range)}累计消耗</CardDescription>
              <CardTitle className="sr-only">所选范围累计消耗</CardTitle>
            </div>
            <CalendarDays className="h-5 w-5 text-sky-600" />
          </CardHeader>
          <CardContent>
            {metricValue(currency(dailyUsage?.total_used_quota_usd))}
            <p className="mt-2 text-xs text-muted-foreground">其中 {activeUsageDays} 天检测到额度增长</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="mb-3">
            <div>
              <CardDescription>日均消耗</CardDescription>
              <CardTitle className="sr-only">日均消耗额度</CardTitle>
            </div>
            <Activity className="h-5 w-5 text-amber-600" />
          </CardHeader>
          <CardContent>
            {metricValue(currency(averageDailyUsage))}
            <p className="mt-2 text-xs text-muted-foreground">按所选自然日数量计算，包含零消耗日期</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="mb-3">
            <div>
              <CardDescription>上游账号</CardDescription>
              <CardTitle className="sr-only">上游账号数量</CardTitle>
            </div>
            <Layers3 className="h-5 w-5 text-violet-600" />
          </CardHeader>
          <CardContent>
            {metricValue(summary?.accounts_total ?? 0)}
            <p className="mt-2 text-xs text-muted-foreground">{summary?.accounts_enabled ?? 0} 个账号已启用定时检测</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="mb-3">
            <div>
              <CardDescription>低余额账号</CardDescription>
              <CardTitle className="sr-only">低余额账号数量</CardTitle>
            </div>
            <WalletCards className="h-5 w-5 text-orange-600" />
          </CardHeader>
          <CardContent>
            {metricValue(summary?.low_balance_accounts ?? 0)}
            <p className="mt-2 text-xs text-muted-foreground">余额已达到配置的预警阈值</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="mb-3">
            <div>
              <CardDescription>检测异常</CardDescription>
              <CardTitle className="sr-only">检测异常账号数量</CardTitle>
            </div>
            <ShieldAlert className="h-5 w-5 text-red-600" />
          </CardHeader>
          <CardContent>
            {metricValue(summary?.failed_accounts ?? 0)}
            <p className="mt-2 text-xs text-muted-foreground">最近一次检测未成功完成的账号</p>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1.5fr)_minmax(320px,0.7fr)]">
        <Card className="min-w-0">
          <CardHeader>
            <div>
              <CardTitle>每日额度消耗</CardTitle>
              <CardDescription>按应用时区汇总所有账号的实际消耗；新版 Sub2API 使用趋势接口，旧版回退到密钥快照差值。</CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            {loading ? (
              <Skeleton className="h-[300px] w-full" />
            ) : hasUsageData ? (
              <ChartContainer config={dailyUsageChartConfig} className="h-[300px] w-full">
                <BarChart accessibilityLayer data={usagePoints} margin={{ left: 4, right: 4, top: 12 }}>
                  <CartesianGrid vertical={false} />
                  <XAxis
                    dataKey="date"
                    tickLine={false}
                    axisLine={false}
                    tickMargin={10}
                    minTickGap={24}
                    tickFormatter={(value: string) => formatDay(value)}
                  />
                  <YAxis
                    tickLine={false}
                    axisLine={false}
                    width={64}
                    tickFormatter={(value: number) => `$${value.toFixed(value < 1 ? 2 : 0)}`}
                  />
                  <ChartTooltip
                    cursor={false}
                    content={
                      <ChartTooltipContent
                        labelFormatter={(value) => formatDay(String(value), true)}
                        formatter={(value) => currency(Number(value))}
                      />
                    }
                  />
                  <Bar dataKey="used_quota_usd" fill="var(--color-used_quota_usd)" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ChartContainer>
            ) : (
              <div className="flex h-[300px] items-center justify-center text-sm text-muted-foreground">暂无可计算的额度快照</div>
            )}
          </CardContent>
        </Card>

        <Card className="min-w-0">
          <CardHeader>
            <div>
              <CardTitle>每日明细</CardTitle>
              <CardDescription>最近 {Math.min(10, usagePoints.length)} 个自然日。</CardDescription>
            </div>
          </CardHeader>
          <CardContent className="overflow-x-auto p-0">
            {loading ? (
              <div className="space-y-3 p-4">
                {Array.from({ length: 6 }).map((_, index) => (
                  <Skeleton key={index} className="h-10 w-full" />
                ))}
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>日期</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead className="text-right">消耗额度</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {recentUsagePoints.map((point) => (
                    <TableRow key={point.date}>
                      <TableCell className="font-medium">{formatDay(point.date, true)}</TableCell>
                      <TableCell>
                        <Badge variant={point.used_quota_usd > 0 ? "success" : "outline"}>
                          {point.used_quota_usd > 0 ? "有消耗" : "无增量"}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right font-mono">{currency(point.used_quota_usd)}</TableCell>
                    </TableRow>
                  ))}
                  {!recentUsagePoints.length ? (
                    <TableRow>
                      <TableCell colSpan={3} className="h-24 text-center text-muted-foreground">暂无每日数据</TableCell>
                    </TableRow>
                  ) : null}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>

      <Card className="min-w-0">
        <CardHeader className="flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <CardTitle>账号余额趋势</CardTitle>
            <CardDescription>对比各上游账号在所选时间范围内的余额变化。</CardDescription>
          </div>
          <Tabs value={chartGranularity} onValueChange={(value) => setChartGranularity(value as ChartGranularity)}>
            <TabsList>
              {chartGranularityOptions.map((item) => (
                <TabsTrigger key={item.value} value={item.value}>
                  {item.label}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        </CardHeader>
        <CardContent>
          {loading ? (
            <Skeleton className="h-[320px] w-full" />
          ) : balanceChartData.length ? (
            <BalanceTrendChart data={balanceChartData} granularity={chartGranularity} />
          ) : (
            <div className="flex h-[320px] items-center justify-center text-sm text-muted-foreground">暂无余额趋势数据</div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
