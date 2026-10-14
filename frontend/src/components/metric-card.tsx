import { ArrowDownRight, ArrowUpRight, LucideIcon } from "lucide-react";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

interface MetricCardProps {
  title: string;
  value: string | number;
  subtitle: string;
  icon: LucideIcon;
  tone?: "default" | "warning" | "danger";
}

export function MetricCard({
  title,
  value,
  subtitle,
  icon: Icon,
  tone = "default",
}: MetricCardProps) {
  return (
    <Card className="group overflow-hidden transition duration-300 hover:-translate-y-0.5 hover:shadow-[0_16px_40px_rgba(0,0,0,0.08)]">
      <CardHeader>
        <div>
          <p className="text-[13px] font-medium tracking-wide text-muted-foreground">{title}</p>
          <CardTitle className="mt-2 font-mono text-[32px] leading-none tracking-tight">{value}</CardTitle>
        </div>
        <div
          className={cn(
            "rounded-2xl p-3",
            tone === "warning" && "bg-amber-500/10 text-amber-500",
            tone === "danger" && "bg-red-500/10 text-red-500",
            tone === "default" && "bg-secondary text-secondary-foreground",
          )}
        >
          <Icon className="h-6 w-6" />
        </div>
      </CardHeader>
      <CardContent className="flex items-center gap-2 text-sm text-muted-foreground">
        {tone === "danger" ? <ArrowDownRight className="h-4 w-4 text-red-600" /> : <ArrowUpRight className="h-4 w-4 text-primary" />}
        <span>{subtitle}</span>
      </CardContent>
    </Card>
  );
}
