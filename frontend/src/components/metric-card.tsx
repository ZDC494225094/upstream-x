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
    <Card className="overflow-hidden">
      <CardHeader>
        <div>
          <p className="text-sm text-muted-foreground">{title}</p>
          <CardTitle className="mt-2 text-3xl">{value}</CardTitle>
        </div>
        <div
          className={cn(
            "rounded-md p-3",
            tone === "warning" && "bg-muted text-foreground",
            tone === "danger" && "bg-muted text-foreground",
            tone === "default" && "bg-muted text-foreground",
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
