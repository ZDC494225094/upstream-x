import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function currency(value?: number | null) {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return "--";
  }
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 2,
    maximumFractionDigits: 4,
  }).format(value);
}

export function formatDateTime(value?: string | null) {
  if (!value) return "--";
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

export function getIntervalBucketTime(value: string | number | Date, intervalMinutes: number) {
  const date = new Date(value);
  const minutes = Math.max(1, intervalMinutes || 1);
  const bucketMs = Math.floor(date.getTime() / (minutes * 60 * 1000)) * minutes * 60 * 1000;
  return new Date(bucketMs).toISOString();
}

export type ChartGranularity = "hour" | "day" | "month";

export function getChartBucketTime(value: string | number | Date, granularity: ChartGranularity) {
  const date = new Date(value);
  const bucket = new Date(date);

  if (granularity === "month") {
    bucket.setDate(1);
    bucket.setHours(0, 0, 0, 0);
    return bucket.toISOString();
  }

  if (granularity === "day") {
    bucket.setHours(0, 0, 0, 0);
    return bucket.toISOString();
  }

  bucket.setMinutes(0, 0, 0);
  return bucket.toISOString();
}

export function formatChartTime(value?: string | number | Date | null) {
  if (!value) return "--";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(new Date(value));
}

export function formatChartBucketLabel(value: string | number | Date, granularity: ChartGranularity) {
  const date = new Date(value);

  if (granularity === "month") {
    return new Intl.DateTimeFormat("zh-CN", {
      year: "numeric",
      month: "2-digit",
    }).format(date);
  }

  if (granularity === "day") {
    return new Intl.DateTimeFormat("zh-CN", {
      month: "2-digit",
      day: "2-digit",
    }).format(date);
  }

  return formatChartTime(date);
}

export function formatDuration(ms?: number | null) {
  if (!ms) return "--";
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

export function formatRangeLabel(range: string) {
  if (range === "3d") return "3天";
  if (range === "7d") return "7天";
  if (range === "30d") return "30天";
  if (range === "90d") return "90天";
  if (range === "all") return "全部";
  return range;
}

export function formatRunStatus(status?: string | null) {
  if (status === "success") return "成功";
  if (status === "failed") return "失败";
  return "未知";
}

export function formatTriggeredBy(triggeredBy?: string | null) {
  if (triggeredBy === "manual") return "手动";
  if (triggeredBy === "scheduler") return "定时任务";
  return triggeredBy || "--";
}
