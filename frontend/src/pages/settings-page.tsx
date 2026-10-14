import { ChangeEvent, FormEvent, useEffect, useRef, useState } from "react";
import { Download, Globe, Link2, Mail, Plus, Save, Send, TimerReset, Trash2, Upload } from "lucide-react";

import { SettingsImportDialog } from "@/components/settings-import-dialog";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ErrorAlert } from "@/components/ui/error-alert";
import { useCustomMenuLinks } from "@/hooks/use-custom-menu-links";
import { usePageRefresh } from "@/hooks/use-page-refresh";
import { useToast } from "@/hooks/use-toast";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/lib/api";
import type { CustomMenuLink, SMTPSettings } from "@/lib/types";

const defaultSMTP: SMTPSettings = {
  enabled: false,
  host: "",
  port: 465,
  username: "",
  password: "",
  from: "",
  recipients: [],
  use_tls: true,
};

function createCustomMenuLinkDraft(): CustomMenuLink {
  return {
    id: `menu-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    title: "",
    url: "",
    embed_iframe: true,
  };
}

export function SettingsPage() {
  const { refreshKey } = usePageRefresh();
  const { setLinks: setGlobalCustomMenuLinks } = useCustomMenuLinks();
  const { toast } = useToast();
  const [schedule, setSchedule] = useState(10);
  const [logRetentionDays, setLogRetentionDays] = useState(90);
  const [smtp, setSMTP] = useState<SMTPSettings>(defaultSMTP);
  const [customMenuLinks, setCustomMenuLinks] = useState<CustomMenuLink[]>([]);
  const [recipientsInput, setRecipientsInput] = useState("");
  const [loading, setLoading] = useState(true);
  const [savingSchedule, setSavingSchedule] = useState(false);
  const [savingLogs, setSavingLogs] = useState(false);
  const [cleaningLogs, setCleaningLogs] = useState(false);
  const [savingSMTP, setSavingSMTP] = useState(false);
  const [savingCustomMenuLinks, setSavingCustomMenuLinks] = useState(false);
  const [testingSMTP, setTestingSMTP] = useState(false);
  const [proxyURL, setProxyURL] = useState("");
  const [savingProxy, setSavingProxy] = useState(false);
  const [testingProxy, setTestingProxy] = useState(false);
  const [proxyTestResult, setProxyTestResult] = useState<string | null>(null);
  const [importOpen, setImportOpen] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const exportAnchorRef = useRef<HTMLAnchorElement | null>(null);

  async function load() {
    setLoading(true);
    try {
      const settings = await api.getSettings();
      setSchedule(settings.schedule_interval_minutes);
      setLogRetentionDays(settings.log_retention_days);
      setSMTP(settings.smtp);
      setCustomMenuLinks(settings.custom_menu_links ?? []);
      setGlobalCustomMenuLinks(settings.custom_menu_links ?? []);
      setRecipientsInput(settings.smtp.recipients.join("\n"));
      setProxyURL(settings.proxy_url ?? "");
      setError(null);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "加载设置失败");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [refreshKey]);

  function normalizeSMTPPayload(): SMTPSettings {
    return {
      ...smtp,
      host: smtp.host.trim(),
      username: smtp.username.trim(),
      from: smtp.from.trim(),
      recipients: recipientsInput
        .split("\n")
        .map((item) => item.trim())
        .filter(Boolean),
    };
  }

  async function handleSaveSchedule() {
    setSavingSchedule(true);
    setError(null);
    try {
      await api.updateSchedule({ interval_minutes: Number(schedule) });
      toast({ title: "调度设置已更新" });
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "保存调度设置失败");
    } finally {
      setSavingSchedule(false);
    }
  }

  async function handleSaveLogRetention() {
    setSavingLogs(true);
    setError(null);
    try {
      await api.updateLogRetention({ log_retention_days: Number(logRetentionDays) });
      toast({ title: "日志保留设置已更新" });
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "保存日志保留设置失败");
    } finally {
      setSavingLogs(false);
    }
  }

  async function handleCleanupLogs() {
    if (!window.confirm(`确认立即清理 ${logRetentionDays} 天之前的历史日志吗？该操作不会删除账号，但会删除历史检测、快照和告警记录。`)) {
      return;
    }
    setCleaningLogs(true);
    setError(null);
    try {
      const result = await api.cleanupLogs({ log_retention_days: Number(logRetentionDays) });
      toast({ title: `历史日志已清理，共删除 ${result.deleted_total} 条记录。` });
    } catch (cleanupError) {
      setError(cleanupError instanceof Error ? cleanupError.message : "清理历史日志失败");
    } finally {
      setCleaningLogs(false);
    }
  }

  async function handleSaveSMTP(event: FormEvent) {
    event.preventDefault();
    setSavingSMTP(true);
    setError(null);
    try {
      const payload = normalizeSMTPPayload();
      await api.updateSMTP(payload);
      setSMTP(payload);
      toast({ title: "SMTP 设置已保存" });
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "保存 SMTP 设置失败");
    } finally {
      setSavingSMTP(false);
    }
  }

  async function handleTestSMTP() {
    setTestingSMTP(true);
    setError(null);
    try {
      const payload = normalizeSMTPPayload();
      await api.updateSMTP(payload);
      setSMTP(payload);
      await api.sendSMTPTest({});
      toast({ title: "测试邮件发送成功" });
    } catch (testError) {
      setError(testError instanceof Error ? testError.message : "测试邮件发送失败");
    } finally {
      setTestingSMTP(false);
    }
  }

  async function handleSaveProxy(event: FormEvent) {
    event.preventDefault();
    setSavingProxy(true);
    setError(null);
    try {
      const result = await api.updateProxy({ proxy_url: proxyURL.trim() });
      setProxyURL(result.proxy_url);
      setProxyTestResult(null);
      toast({ title: result.proxy_url ? "代理设置已保存，立即生效" : "代理已清空，后续请求直连" });
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "保存代理设置失败");
    } finally {
      setSavingProxy(false);
    }
  }

  async function handleTestProxy() {
    setTestingProxy(true);
    setError(null);
    setProxyTestResult(null);
    try {
      const result = await api.testProxy({ proxy_url: proxyURL.trim() });
      setProxyTestResult(
        result.proxy_used
          ? `连通成功，当前通过代理的出口 IP：${result.ip}`
          : `当前为直连，出口 IP：${result.ip}`
      );
    } catch (testError) {
      setError(testError instanceof Error ? testError.message : "代理连通性测试失败");
    } finally {
      setTestingProxy(false);
    }
  }

  async function handleSaveCustomMenuLinks() {
    setSavingCustomMenuLinks(true);
    setError(null);
    try {
      const payload = {
        items: customMenuLinks.map((item) => ({
          id: item.id,
          title: item.title.trim(),
          url: item.url.trim(),
          embed_iframe: item.embed_iframe,
        })),
      };
      const result = await api.updateCustomMenuLinks(payload);
      setCustomMenuLinks(result.items);
      setGlobalCustomMenuLinks(result.items);
      toast({ title: "自定义菜单已保存" });
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "保存自定义菜单失败");
    } finally {
      setSavingCustomMenuLinks(false);
    }
  }

  async function handleExportSettings() {
    setExporting(true);
    setError(null);
    try {
      const bundle = await api.exportSettings();
      const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: "application/json" });
      const url = window.URL.createObjectURL(blob);

      if (!exportAnchorRef.current) {
        exportAnchorRef.current = document.createElement("a");
      }

      const anchor = exportAnchorRef.current;
      const date = new Date().toISOString().slice(0, 19).replace(/[:T]/g, "-");
      anchor.href = url;
      anchor.download = `upstream-x-settings-${date}.json`;
      anchor.click();
      window.setTimeout(() => window.URL.revokeObjectURL(url), 1000);
      toast({ title: "系统设置 JSON 已导出" });
    } catch (exportError) {
      setError(exportError instanceof Error ? exportError.message : "导出系统设置失败");
      toast({ title: "系统设置导出失败", variant: "error" });
    } finally {
      setExporting(false);
    }
  }

  async function handleImportSettings(payload: unknown) {
    const snapshot = await api.importSettings(payload);
    setSchedule(snapshot.schedule_interval_minutes);
    setLogRetentionDays(snapshot.log_retention_days);
    setSMTP(snapshot.smtp);
    setCustomMenuLinks(snapshot.custom_menu_links ?? []);
    setGlobalCustomMenuLinks(snapshot.custom_menu_links ?? []);
    setRecipientsInput(snapshot.smtp.recipients.join("\n"));
    setProxyURL(snapshot.proxy_url ?? "");
    setError(null);
    toast({ title: "系统设置已导入并生效" });
    return snapshot;
  }

  if (loading) {
    return <div className="text-sm text-muted-foreground">加载设置中...</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="text-3xl font-semibold">系统设置</h1>
          <p className="mt-2 text-sm text-muted-foreground">运行时可调整的配置统一放在这里，保存后立即生效。</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" onClick={() => void handleExportSettings()} disabled={exporting}>
            <Download className="mr-2 h-4 w-4" />
            {exporting ? "导出中..." : "导出配置"}
          </Button>
          <Button variant="outline" onClick={() => setImportOpen(true)}>
            <Upload className="mr-2 h-4 w-4" />
            导入配置
          </Button>
        </div>
      </div>

      {error && <ErrorAlert title="系统设置操作失败" message={error} />}

      <Tabs defaultValue="schedule" className="space-y-4">
        <TabsList className="flex w-full flex-wrap justify-start rounded-lg bg-muted p-1">
          <TabsTrigger className="rounded-md" value="schedule">调度设置</TabsTrigger>
          <TabsTrigger className="rounded-md" value="logs">日志管理</TabsTrigger>
          <TabsTrigger className="rounded-md" value="menu-links">自定义菜单</TabsTrigger>
          <TabsTrigger className="rounded-md" value="smtp">SMTP</TabsTrigger>
          <TabsTrigger className="rounded-md" value="proxy">上游代理</TabsTrigger>
        </TabsList>

        <TabsContent value="schedule">
          <Card>
            <CardHeader>
              <div className="flex items-center gap-3">
                <div className="rounded-md bg-secondary p-2">
                  <TimerReset className="h-5 w-5" />
                </div>
                <div>
                  <CardTitle>调度设置</CardTitle>
                  <CardDescription>修改定时检测周期，保存后会热更新调度器。</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="max-w-sm space-y-2">
                <Label htmlFor="schedule-interval">检测间隔（分钟）</Label>
                <Input
                  id="schedule-interval"
                  type="number"
                  min="1"
                  value={schedule}
                  onChange={(e: ChangeEvent<HTMLInputElement>) => setSchedule(Number(e.target.value))}
                />
              </div>
              <Button onClick={handleSaveSchedule} disabled={savingSchedule}>
                <Save className="mr-2 h-4 w-4" />
                {savingSchedule ? "保存中..." : "保存调度设置"}
              </Button>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="logs">
          <Card>
            <CardHeader>
              <div>
                <CardTitle>日志管理</CardTitle>
                <CardDescription>设置历史日志保留时间，并支持立即清理超出保留周期的检测记录、快照和告警事件。</CardDescription>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="max-w-sm space-y-2">
                <Label htmlFor="log-retention-days">日志保留天数</Label>
                <Input
                  id="log-retention-days"
                  type="number"
                  min="1"
                  value={logRetentionDays}
                  onChange={(e: ChangeEvent<HTMLInputElement>) => setLogRetentionDays(Number(e.target.value))}
                />
              </div>
              <div className="flex flex-wrap gap-3">
                <Button onClick={handleSaveLogRetention} disabled={savingLogs}>
                  <Save className="mr-2 h-4 w-4" />
                  {savingLogs ? "保存中..." : "保存日志策略"}
                </Button>
                <Button type="button" variant="outline" onClick={() => void handleCleanupLogs()} disabled={cleaningLogs}>
                  <Trash2 className="mr-2 h-4 w-4" />
                  {cleaningLogs ? "清理中..." : "立即清理历史日志"}
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">清理范围包括检测任务、余额快照、分组快照、令牌快照和告警记录，不会删除账号本身。系统也会在每轮定时检测结束后，自动按这里设置的保留时间清理过期日志。</p>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="menu-links">
          <Card>
            <CardHeader>
              <div className="flex items-center gap-3">
                <div className="rounded-md bg-secondary p-2">
                  <Link2 className="h-5 w-5" />
                </div>
                <div>
                  <CardTitle>自定义菜单</CardTitle>
                  <CardDescription>允许在左侧菜单增加自定义入口，可直接跳转，也可以在后台内用 iframe 打开。</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              {customMenuLinks.length ? (
                <div className="space-y-4">
                  {customMenuLinks.map((item) => (
                    <div key={item.id} className="rounded-lg border p-4">
                      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)_auto]">
                        <div className="space-y-2">
                          <Label htmlFor={`menu-title-${item.id}`}>菜单名称</Label>
                          <Input
                            id={`menu-title-${item.id}`}
                            value={item.title}
                            onChange={(e: ChangeEvent<HTMLInputElement>) =>
                              setCustomMenuLinks((current) =>
                                current.map((link) => (link.id === item.id ? { ...link, title: e.target.value } : link)),
                              )
                            }
                          />
                        </div>
                        <div className="space-y-2">
                          <Label htmlFor={`menu-url-${item.id}`}>链接地址</Label>
                          <Input
                            id={`menu-url-${item.id}`}
                            placeholder="https://example.com 或 /internal/path"
                            value={item.url}
                            onChange={(e: ChangeEvent<HTMLInputElement>) =>
                              setCustomMenuLinks((current) =>
                                current.map((link) => (link.id === item.id ? { ...link, url: e.target.value } : link)),
                              )
                            }
                          />
                        </div>
                        <div className="flex items-end justify-end">
                          <Button
                            type="button"
                            variant="outline"
                            onClick={() =>
                              setCustomMenuLinks((current) => current.filter((link) => link.id !== item.id))
                            }
                          >
                            <Trash2 className="mr-2 h-4 w-4" />
                            删除
                          </Button>
                        </div>
                      </div>

                      <div className="mt-4 flex items-center justify-between rounded-lg border bg-muted/20 px-4 py-3">
                        <div>
                          <Label htmlFor={`menu-iframe-${item.id}`}>在后台内嵌 iframe 打开</Label>
                          <p className="mt-1 text-xs text-muted-foreground">开启后点击菜单会进入后台内嵌页面；若目标站点禁止 iframe，页面可能无法显示。</p>
                        </div>
                        <Switch
                          id={`menu-iframe-${item.id}`}
                          checked={item.embed_iframe}
                          onCheckedChange={(checked) =>
                            setCustomMenuLinks((current) =>
                              current.map((link) => (link.id === item.id ? { ...link, embed_iframe: checked } : link)),
                            )
                          }
                        />
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="rounded-lg border border-dashed px-4 py-8 text-sm text-muted-foreground">
                  还没有自定义菜单链接，点击下方按钮可以新增一个。
                </div>
              )}

              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="text-xs text-muted-foreground">
                  支持多个菜单项。相对路径会按当前站点跳转，绝对外链会新标签打开；开启 iframe 后会以内嵌页面形式展示。
                </div>
                <div className="flex flex-wrap gap-3">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => setCustomMenuLinks((current) => [...current, createCustomMenuLinkDraft()])}
                  >
                    <Plus className="mr-2 h-4 w-4" />
                    新增菜单
                  </Button>
                  <Button type="button" onClick={() => void handleSaveCustomMenuLinks()} disabled={savingCustomMenuLinks}>
                    <Save className="mr-2 h-4 w-4" />
                    {savingCustomMenuLinks ? "保存中..." : "保存自定义菜单"}
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="smtp">
          <Card>
            <CardHeader>
              <div className="flex items-center gap-3">
                <div className="rounded-md bg-secondary p-2">
                  <Mail className="h-5 w-5" />
                </div>
                <div>
                  <CardTitle>SMTP 设置</CardTitle>
                  <CardDescription>用于余额告警、异常恢复和分组变更通知，也支持发送测试邮件。</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <form className="space-y-5" onSubmit={handleSaveSMTP}>
                <div className="flex items-center justify-between rounded-lg border bg-muted/20 px-4 py-3">
                  <div>
                    <Label htmlFor="smtp-enabled">启用 SMTP</Label>
                    <p className="mt-1 text-xs text-muted-foreground">关闭后系统仍会记录日志，但不会发送邮件通知。</p>
                  </div>
                  <Switch
                    id="smtp-enabled"
                    checked={smtp.enabled}
                    onCheckedChange={(checked) => setSMTP((current) => ({ ...current, enabled: checked }))}
                  />
                </div>

                <div className="grid gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="smtp-host">SMTP Host</Label>
                    <Input
                      id="smtp-host"
                      value={smtp.host}
                      onChange={(e: ChangeEvent<HTMLInputElement>) => setSMTP((current) => ({ ...current, host: e.target.value }))}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="smtp-port">SMTP Port</Label>
                    <Input
                      id="smtp-port"
                      type="number"
                      min="1"
                      value={smtp.port}
                      onChange={(e: ChangeEvent<HTMLInputElement>) => setSMTP((current) => ({ ...current, port: Number(e.target.value) }))}
                    />
                  </div>
                </div>

                <div className="grid gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="smtp-username">SMTP 用户名</Label>
                    <Input
                      id="smtp-username"
                      value={smtp.username}
                      onChange={(e: ChangeEvent<HTMLInputElement>) => setSMTP((current) => ({ ...current, username: e.target.value }))}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="smtp-password">SMTP 密码</Label>
                    <Input
                      id="smtp-password"
                      type="password"
                      value={smtp.password}
                      onChange={(e: ChangeEvent<HTMLInputElement>) => setSMTP((current) => ({ ...current, password: e.target.value }))}
                    />
                  </div>
                </div>

                <div className="grid gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="smtp-from">发件人</Label>
                    <Input
                      id="smtp-from"
                      value={smtp.from}
                      onChange={(e: ChangeEvent<HTMLInputElement>) => setSMTP((current) => ({ ...current, from: e.target.value }))}
                    />
                  </div>
                  <div className="flex items-end">
                    <div className="flex w-full items-center justify-between rounded-lg border bg-muted/20 px-4 py-3">
                      <div>
                        <Label htmlFor="smtp-use-tls">使用 TLS</Label>
                        <p className="mt-1 text-xs text-muted-foreground">465 等隐式 TLS 场景建议开启。</p>
                      </div>
                      <Switch
                        id="smtp-use-tls"
                        checked={smtp.use_tls}
                        onCheckedChange={(checked) => setSMTP((current) => ({ ...current, use_tls: checked }))}
                      />
                    </div>
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="smtp-recipients">收件人列表</Label>
                  <Textarea
                    id="smtp-recipients"
                    placeholder={"每行一个邮箱地址\nops@example.com\nalert@example.com"}
                    value={recipientsInput}
                    onChange={(e: ChangeEvent<HTMLTextAreaElement>) => setRecipientsInput(e.target.value)}
                  />
                </div>

                <div className="flex flex-wrap justify-end gap-3">
                  <div className="mr-auto self-center text-xs text-muted-foreground">
                    测试邮件会直接使用当前表单配置发送，不受“启用 SMTP”开关影响。
                  </div>
                  <Button type="button" variant="outline" onClick={() => void handleTestSMTP()} disabled={testingSMTP}>
                    <Send className="mr-2 h-4 w-4" />
                    {testingSMTP ? "发送中..." : "发送测试邮件"}
                  </Button>
                  <Button type="submit" disabled={savingSMTP}>
                    <Save className="mr-2 h-4 w-4" />
                    {savingSMTP ? "保存中..." : "保存 SMTP 设置"}
                  </Button>
                </div>
              </form>
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="proxy">
          <Card>
            <CardHeader>
              <div className="flex items-center gap-3">
                <div className="rounded-md bg-secondary p-2">
                  <Globe className="h-5 w-5" />
                </div>
                <div>
                  <CardTitle>上游代理</CardTitle>
                  <CardDescription>所有访问上游站点的请求（登录、刷新 token、余额检测）都经此代理出口。留空为直连。</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <form className="space-y-5" onSubmit={handleSaveProxy}>
                <div className="space-y-2">
                  <Label htmlFor="proxy-url">代理地址</Label>
                  <Input
                    id="proxy-url"
                    placeholder="socks5://用户名:密码@主机:端口，如 socks5://127.0.0.1:1080"
                    value={proxyURL}
                    onChange={(e: ChangeEvent<HTMLInputElement>) => setProxyURL(e.target.value)}
                  />
                  <p className="text-xs leading-relaxed text-muted-foreground">
                    支持 socks5 / socks5h / http / https。适用场景：上游开启会话 IP 绑定或人机验证 IP 校验时，浏览器登录的 IP 与本服务部署服务器的 IP 不一致会被判 401
                    （如 SESSION_BINDING_MISMATCH）。配置一台与浏览器同出口的代理（例如同一台机器或同一家庭宽带出口），让本服务的登录与检测都走同一个 IP
                    即可对齐。保存后立即生效，无需重启。注意：代理本身无法替你通过人机验证，开了验证码的上游仍需账号密码模式下上游侧放行或改用 refresh token 模式。
                  </p>
                </div>

                {proxyTestResult ? (
                  <div className="rounded-lg border bg-muted/20 px-4 py-3 text-sm">{proxyTestResult}</div>
                ) : null}

                <div className="flex flex-wrap justify-end gap-3">
                  <Button type="button" variant="outline" onClick={() => void handleTestProxy()} disabled={testingProxy}>
                    <Globe className="mr-2 h-4 w-4" />
                    {testingProxy ? "测试中..." : "测试出口 IP"}
                  </Button>
                  <Button type="submit" disabled={savingProxy}>
                    <Save className="mr-2 h-4 w-4" />
                    {savingProxy ? "保存中..." : "保存代理设置"}
                  </Button>
                </div>
              </form>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
      <SettingsImportDialog open={importOpen} onOpenChange={setImportOpen} onImport={handleImportSettings} />
    </div>
  );
}
