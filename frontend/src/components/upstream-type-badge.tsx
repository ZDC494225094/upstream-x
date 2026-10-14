import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { UpstreamType } from "@/lib/types";

interface UpstreamTypeBadgeProps {
  type?: UpstreamType | null;
  className?: string;
}

function formatUpstreamType(type?: UpstreamType | null) {
  return type === "sub2api" ? "Sub2API" : "New API";
}

function getUpstreamTypeLogo(type?: UpstreamType | null) {
  if (type === "sub2api") {
    return "/sub2api.logo.png";
  }
  if (type === "newapi") {
    return "/newapi.svg";
  }
  return null;
}

export function UpstreamTypeBadge({ type, className }: UpstreamTypeBadgeProps) {
  const logo = getUpstreamTypeLogo(type);

  return (
    <Badge variant="outline" className={cn("gap-1.5 pl-1.5 pr-2.5", className)}>
      {logo ? (
        <img
          src={logo}
          alt=""
          aria-hidden="true"
          className="h-4 w-4 shrink-0 rounded-sm object-contain"
        />
      ) : null}
      <span>{formatUpstreamType(type)}</span>
    </Badge>
  );
}
