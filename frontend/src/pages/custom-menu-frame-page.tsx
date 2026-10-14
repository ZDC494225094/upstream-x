import { ExternalLink, Link2 } from "lucide-react";
import { useParams } from "react-router-dom";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ErrorAlert } from "@/components/ui/error-alert";
import { useCustomMenuLinks } from "@/hooks/use-custom-menu-links";

export function CustomMenuFramePage() {
  const { id } = useParams();
  const { links, loading } = useCustomMenuLinks();
  const link = links.find((item) => item.id === id);

  if (loading) {
    return <div className="text-sm text-muted-foreground">加载页面中...</div>;
  }

  if (!link) {
    return <ErrorAlert title="菜单页面不存在" message="未找到对应的自定义菜单链接，请返回系统设置检查配置。" />;
  }

  const isExternal = !link.url.startsWith("/");

  if (!link.embed_iframe) {
    return (
      <Card>
        <CardHeader>
          <div className="flex items-center gap-3">
            <div className="rounded-md bg-secondary p-2">
              <Link2 className="h-5 w-5" />
            </div>
            <div>
              <CardTitle>{link.title}</CardTitle>
              <CardDescription>这个菜单未启用 iframe 内嵌，请直接打开目标链接。</CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="rounded-lg border bg-muted/20 px-4 py-3 font-mono text-xs text-muted-foreground">
            {link.url}
          </div>
          <Button asChild>
            <a href={link.url} target={isExternal ? "_blank" : undefined} rel={isExternal ? "noreferrer" : undefined}>
              <ExternalLink className="mr-2 h-4 w-4" />
              打开链接
            </a>
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="text-3xl font-semibold">{link.title}</h1>
          <p className="mt-2 break-all font-mono text-xs text-muted-foreground">{link.url}</p>
        </div>
        <Button variant="outline" asChild>
          <a href={link.url} target={isExternal ? "_blank" : undefined} rel={isExternal ? "noreferrer" : undefined}>
            <ExternalLink className="mr-2 h-4 w-4" />
            新标签打开
          </a>
        </Button>
      </div>

      <div className="overflow-hidden rounded-lg border bg-card shadow-sm">
        <iframe
          src={link.url}
          title={link.title}
          className="h-[70vh] w-full bg-background md:h-[calc(100vh-15rem)]"
        />
      </div>

      <p className="text-xs text-muted-foreground">
        如果目标站点设置了 X-Frame-Options 或 Content-Security-Policy，浏览器可能会阻止 iframe 显示，这时请使用“新标签打开”。
      </p>
    </div>
  );
}
