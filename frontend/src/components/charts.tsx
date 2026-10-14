import { useMemo } from "react";
import { CartesianGrid, Line, LineChart, XAxis, YAxis } from "recharts";

import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import { type ChartGranularity, currency, formatChartBucketLabel, getChartBucketTime } from "@/lib/utils";

type ChartPoint = Record<string, string | number>;

const chartColors = [
  "hsl(160 84% 30%)",
  "hsl(25 95% 53%)",
  "hsl(221 83% 53%)",
  "hsl(45 93% 47%)",
  "hsl(262 83% 58%)",
  "hsl(0 72% 51%)",
  "hsl(190 90% 35%)",
  "hsl(330 81% 60%)",
];

type PreparedSeries = {
  key: string;
  label: string;
  color: string;
};

function normalizeBalanceSeries(data: ChartPoint[], granularity: ChartGranularity) {
  const grouped = new Map<string, ChartPoint>();
  for (const row of data) {
    const rawTime = typeof row.rawTime === "string" ? row.rawTime : String(row.time ?? "");
    const bucketTime = getChartBucketTime(rawTime, granularity);
    const nextRow = grouped.get(bucketTime) ?? {
      rawTime: bucketTime,
      time: formatChartBucketLabel(bucketTime, granularity),
    };
    for (const [key, value] of Object.entries(row)) {
      if (key !== "time" && key !== "rawTime") nextRow[key] = value;
    }
    grouped.set(bucketTime, nextRow);
  }
  return Array.from(grouped.entries())
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([, value]) => value);
}

function prepareLineData(rows: ChartPoint[]) {
  const labels = Array.from(
    new Set(rows.flatMap((row) => Object.keys(row).filter((key) => key !== "time" && key !== "rawTime"))),
  ).sort((a, b) => a.localeCompare(b, "zh-CN"));
  const series: PreparedSeries[] = labels.map((label, index) => ({
    key: `series_${index}`,
    label,
    color: chartColors[index % chartColors.length],
  }));
  const chartRows = rows.map((row) => {
    const next: Record<string, string | number | null> = {
      time: String(row.time ?? "--"),
      rawTime: String(row.rawTime ?? ""),
    };
    series.forEach((item) => {
      next[item.key] = row[item.label] == null ? null : Number(row[item.label]);
    });
    return next;
  });
  const config = Object.fromEntries(
    series.map((item) => [item.key, { label: item.label, color: item.color }]),
  ) satisfies ChartConfig;
  return { chartRows, series, config };
}

export function BalanceTrendChart({
  data,
  granularity = "hour",
}: {
  data: Array<Record<string, string | number>>;
  granularity?: ChartGranularity;
}) {
  const prepared = useMemo(
    () => prepareLineData(normalizeBalanceSeries(data, granularity)),
    [data, granularity],
  );

  return (
    <ChartContainer config={prepared.config} className="h-[320px] w-full">
      <LineChart accessibilityLayer data={prepared.chartRows} margin={{ left: 4, right: 12, top: 8 }}>
        <CartesianGrid vertical={false} />
        <XAxis dataKey="time" tickLine={false} axisLine={false} tickMargin={10} minTickGap={24} />
        <YAxis
          tickLine={false}
          axisLine={false}
          width={64}
          tickFormatter={(value: number) => `$${value.toFixed(value < 1 ? 2 : 0)}`}
        />
        <ChartTooltip
          content={<ChartTooltipContent formatter={(value) => currency(Number(value))} />}
        />
        <ChartLegend content={<ChartLegendContent className="pb-3" />} />
        {prepared.series.map((item) => (
          <Line
            key={item.key}
            type="monotone"
            dataKey={item.key}
            name={item.label}
            stroke={`var(--color-${item.key})`}
            strokeWidth={2.25}
            dot={false}
            connectNulls={false}
            isAnimationActive={false}
          />
        ))}
      </LineChart>
    </ChartContainer>
  );
}

function prepareGroupRatioData(
  data: Array<{ run_id: number; time: string; group_name: string; rate_multiplier: number }>,
  granularity: ChartGranularity,
) {
  const grouped = new Map<string, ChartPoint>();
  for (const row of data) {
    const bucketTime = getChartBucketTime(row.time, granularity);
    const bucketKey = `${row.run_id}-${bucketTime}`;
    const current = grouped.get(bucketKey) ?? {
      rawTime: bucketTime,
      time: formatChartBucketLabel(bucketTime, granularity),
    };
    current[row.group_name] = row.rate_multiplier;
    grouped.set(bucketKey, current);
  }
  const rows = Array.from(grouped.values()).sort((left, right) =>
    String(left.rawTime).localeCompare(String(right.rawTime)),
  );
  return prepareLineData(rows);
}

export function GroupRatioChart({
  data,
  granularity = "hour",
}: {
  data: Array<{ run_id: number; time: string; group_name: string; rate_multiplier: number }>;
  granularity?: ChartGranularity;
}) {
  const prepared = useMemo(() => prepareGroupRatioData(data, granularity), [data, granularity]);

  return (
    <ChartContainer config={prepared.config} className="h-[320px] w-full">
      <LineChart accessibilityLayer data={prepared.chartRows} margin={{ left: 4, right: 12, top: 8 }}>
        <CartesianGrid vertical={false} />
        <XAxis dataKey="time" tickLine={false} axisLine={false} tickMargin={10} minTickGap={24} />
        <YAxis tickLine={false} axisLine={false} width={52} tickFormatter={(value: number) => `${value}x`} />
        <ChartTooltip
          content={<ChartTooltipContent formatter={(value) => Number(value).toFixed(4).replace(/0+$/, "").replace(/\.$/, "")} />}
        />
        <ChartLegend content={<ChartLegendContent className="pb-3" />} />
        {prepared.series.map((item) => (
          <Line
            key={item.key}
            type="monotone"
            dataKey={item.key}
            name={item.label}
            stroke={`var(--color-${item.key})`}
            strokeWidth={2.25}
            dot={false}
            connectNulls={false}
            isAnimationActive={false}
          />
        ))}
      </LineChart>
    </ChartContainer>
  );
}
