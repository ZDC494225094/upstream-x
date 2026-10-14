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
    <Card className="group overflow-hidden transition duration-300 hover:-translate-y-0.5 hover:shadow-[0_16px_40px_rgba(2,132,199,0.14)]">
      <CardHeader>
        <div>
          <p className="text-[13px] font-medium tracking-wide text-muted-foreground">{title}</p>
          <CardTitle className="mt-2 font-mono text-[32px] leading-none tracking-tight">{value}</CardTitle>
        </div>
        <div
          className={cn(
            "rounded-xl p-3 ring-1",
            tone === "warning" && "bg-amber-500/10 text-amber-500 ring-amber-500/25",
            tone === "danger" && "bg-red-500/10 text-red-500 ring-red-500/25",
            tone === "default" && "bg-cyan-500/10 text-cyan-500 ring-cyan-500/25",
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
