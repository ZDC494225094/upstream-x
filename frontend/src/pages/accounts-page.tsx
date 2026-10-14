import { useEffect, useRef, useState } from "react";
import { AlertCircle, CheckCircle2, Download, Eye, HelpCircle, Pencil, Plus, RefreshCw, Trash2, Upload } from "lucide-react";
import { Link } from "react-router-dom";

import { AccountImportDialog } from "@/components/account-import-dialog";
import { AccountFormDialog } from "@/components/account-form-dialog";
import { UpstreamTypeBadge } from "@/components/upstream-type-badge";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ErrorAlert } from "@/components/ui/error-alert";
import { usePageRefresh } from "@/hooks/use-page-refresh";
import { useToast } from "@/hooks/use-toast";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api } from "@/lib/api";
import type { Account } from "@/lib/types";
import { currency, formatDateTime } from "@/lib/utils";

function formatAccountStatus(status?: string | null) {
  if (status === "success") return "成功";
  if (status === "failed") return "失败";
  return "未检测";
}

function getAccountStatusVariant(status?: string | null) {
  if (status === "success") return "success" as const;
  if (status === "failed") return "destructive" as const;
  return "default" as const;
}

function getAccountStatusIcon(status?: string | null) {
  if (status === "success") return <CheckCircle2 className="h-3 w-3" />;
  if (status === "failed") return <AlertCircle className="h-3 w-3" />;
  return <HelpCircle className="h-3 w-3" />;
}

export function AccountsPage() {
  const { refreshKey } = usePageRefresh();
  const { toast } = useToast();
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [editing, setEditing] = useState<Account | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<number | null>(null);
  const [exporting, setExporting] = useState(false);
  const exportAnchorRef = useRef<HTMLAnchorElement | null>(null);

  async function load() {
    try {
      const data = await api.listAccounts();
      setAccounts(data);
      setError(null);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "加载账号失败");
    }
  }

  useEffect(() => {
    void load();
  }, [refreshKey]);

  async function handleSubmit(payload: Record<string, unknown>) {
    if (editing) {
      await api.updateAccount(editing.id, payload);
    } else {
      await api.createAccount(payload);
    }
    await load();
    setEditing(null);
  }

  async function handleValidate(payload: Record<string, unknown>) {
    // Sub2API refresh tokens rotate during validation: the response carries
    // the NEW credentials, and the dialog must save those instead of the
    // (now burned) values from the form.
    if (editing) {
      const result = await api.request<import("@/lib/api").ValidateAccountResult>(
        `/api/accounts/${editing.id}/validate`,
        {
          method: "POST",
          body: JSON.stringify(payload),
        },
      );
      return result.credentials;
    }
    const result = await api.validateNewAccount(payload);
    return result.credentials;
  }

  async function handleDelete(id: number) {
    if (!window.confirm("确认删除这个账号吗？历史快照会保留。")) return;
    await api.deleteAccount(id);
    await load();
  }

  async function handleCheck(id: number) {
    setBusyId(id);
    try {
      await api.checkAccount(id);
      await load();
    } catch (checkError) {
      setError(checkError instanceof Error ? checkError.message : "检测失败");
    } finally {
      setBusyId(null);
    }
  }

  async function handleExport() {
    setExporting(true);
    try {
      const bundle = await api.exportAccounts();
      const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: "application/json" });
      const url = window.URL.createObjectURL(blob);

      if (!exportAnchorRef.current) {
        exportAnchorRef.current = document.createElement("a");
      }

      const anchor = exportAnchorRef.current;
      const date = new Date().toISOString().slice(0, 19).replace(/[:T]/g, "-");
      anchor.href = url;
      anchor.download = `upstream-x-accounts-${date}.json`;
      anchor.click();
      window.setTimeout(() => window.URL.revokeObjectURL(url), 1000);
      toast({ title: "账号 JSON 已导出" });
    } catch (exportError) {
      setError(exportError instanceof Error ? exportError.message : "导出失败");
      toast({ title: "账号导出失败", variant: "error" });
    } finally {
      setExporting(false);
    }
  }

  async function handleImport(payload: unknown) {
    const result = await api.importAccounts(payload);
    await load();
    toast({
      title: `导入完成：新增 ${result.created} 个，更新 ${result.updated} 个，失败 ${result.failed} 个`,
      variant: result.failed > 0 ? "error" : "success",
    });
    return result;
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="text-3xl font-semibold">账号管理</h1>
          <p className="mt-2 text-sm text-muted-foreground">新增或维护 `Sub2API` / `New API` 上游账号，并可手动触发检测。</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" onClick={() => void handleExport()} disabled={exporting}>
            <Download className="mr-2 h-4 w-4" />
            {exporting ? "导出中..." : "导出账号"}
          </Button>
          <Button variant="outline" onClick={() => setImportOpen(true)}>
            <Upload className="mr-2 h-4 w-4" />
            导入账号
          </Button>
          <Button onClick={() => setDialogOpen(true)}>
            <Plus className="mr-2 h-4 w-4" />
            添加账号
          </Button>
        </div>
      </div>

      {error && <ErrorAlert title="加载账号失败" message={error} />}

      <Card className="p-4 md:p-6">
        <CardHeader>
          <div>
            <CardTitle>上游账号列表</CardTitle>
            <CardDescription>保存时会即时校验接口可用性，列表状态来自最近一次检测。</CardDescription>
          </div>
        </CardHeader>
        <CardContent className="pt-0">
          <div className="space-y-3 md:hidden">
            {accounts.map((account) => (
              <div key={account.id} className="rounded-lg border bg-background p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <div className="truncate font-medium">{account.name}</div>
                      <span className="text-[11px] text-muted-foreground">ID {account.id}</span>
                    </div>
                    {account.remark ? <div className="mt-1 line-clamp-2 text-xs text-muted-foreground">{account.remark}</div> : null}
                    <div className="mt-1 break-all text-xs text-muted-foreground">{account.base_url}</div>
                  </div>
                  <UpstreamTypeBadge type={account.type} />
                </div>

                <div className="mt-3 flex flex-wrap items-center gap-2">
                  <Badge className="gap-1 px-2 text-[11px]" variant={getAccountStatusVariant(account.last_status)}>
                    {getAccountStatusIcon(account.last_status)}
                    {formatAccountStatus(account.last_status)}
                  </Badge>
                  {!account.enabled && <span className="text-xs text-muted-foreground">已停用</span>}
                </div>

                <div className="mt-4 grid grid-cols-2 gap-3 text-sm">
                  <div>
                    <div className="text-xs text-muted-foreground">余额</div>
                    <div className="mt-1 font-medium">{currency(account.last_balance_usd)}</div>
                  </div>
                  <div>
                    <div className="text-xs text-muted-foreground">阈值</div>
                    <div className="mt-1 font-medium">{currency(account.low_balance_threshold_usd)}</div>
                  </div>
                  <div className="col-span-2">
                    <div className="text-xs text-muted-foreground">最近检测</div>
                    <div className="mt-1 font-medium">{formatDateTime(account.last_checked_at)}</div>
                  </div>
                </div>

                <div className="mt-4 grid grid-cols-2 gap-2">
                  <Button size="sm" variant="outline" className="h-8 px-2.5 text-xs" asChild>
                    <Link to={`/accounts/${account.id}`}>
                      <Eye className="mr-1.5 h-3.5 w-3.5" />
                      详情
                    </Link>
                  </Button>
                  <Button
                    size="sm"
                    variant="secondary"
                    className="h-8 px-2.5 text-xs"
                    onClick={() => void handleCheck(account.id)}
                    disabled={busyId === account.id}
                  >
                    <RefreshCw className={`mr-1.5 h-3.5 w-3.5 ${busyId === account.id ? "animate-spin" : ""}`} />
                    {busyId === account.id ? "检测中..." : "检测"}
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    className="h-8 px-2.5 text-xs"
                    onClick={() => {
                      setEditing(account);
                      setDialogOpen(true);
                    }}
                  >
                    <Pencil className="mr-1.5 h-3.5 w-3.5" />
                    编辑
                  </Button>
                  <Button size="sm" variant="ghost" className="h-8 px-2.5 text-xs" onClick={() => void handleDelete(account.id)}>
                    <Trash2 className="mr-1.5 h-3.5 w-3.5" />
                    删除
                  </Button>
                </div>
              </div>
            ))}
          </div>

          <div className="-mx-3 hidden overflow-x-auto table-scroll-clean md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[88px]">ID</TableHead>
                  <TableHead>名称</TableHead>
                  <TableHead>备注</TableHead>
                  <TableHead>类型</TableHead>
                  <TableHead>余额</TableHead>
                  <TableHead>阈值</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead className="w-[160px] whitespace-nowrap">最近检测</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {accounts.map((account) => (
                  <TableRow key={account.id}>
                    <TableCell className="font-mono text-sm">{account.id}</TableCell>
                    <TableCell>
                      <div className="font-medium">{account.name}</div>
                      <div className="text-xs text-muted-foreground">{account.base_url}</div>
                    </TableCell>
                    <TableCell className="max-w-[260px]">
                      <div className="line-clamp-2 text-sm text-muted-foreground">{account.remark || "--"}</div>
                    </TableCell>
                    <TableCell>
                      <UpstreamTypeBadge type={account.type} />
                    </TableCell>
                    <TableCell>{currency(account.last_balance_usd)}</TableCell>
                    <TableCell>{currency(account.low_balance_threshold_usd)}</TableCell>
                    <TableCell>
                      <div className="flex flex-col gap-1">
                        <Badge className="w-fit gap-1 px-2 text-[11px]" variant={getAccountStatusVariant(account.last_status)}>
                          {getAccountStatusIcon(account.last_status)}
                          {formatAccountStatus(account.last_status)}
                        </Badge>
                        {!account.enabled && <span className="text-xs text-muted-foreground">已停用</span>}
                      </div>
                    </TableCell>
                    <TableCell className="whitespace-nowrap">{formatDateTime(account.last_checked_at)}</TableCell>
                    <TableCell className="w-[1%] whitespace-nowrap">
                      <div className="flex justify-end gap-1.5">
                        <Button size="sm" variant="outline" className="h-8 px-2.5 text-xs" asChild>
                          <Link to={`/accounts/${account.id}`}>
                            <Eye className="mr-1.5 h-3.5 w-3.5" />
                            详情
                          </Link>
                        </Button>
                        <Button
                          size="sm"
                          variant="secondary"
                          className="h-8 px-2.5 text-xs"
                          onClick={() => void handleCheck(account.id)}
                          disabled={busyId === account.id}
                        >
                          <RefreshCw className={`mr-1.5 h-3.5 w-3.5 ${busyId === account.id ? "animate-spin" : ""}`} />
                          {busyId === account.id ? "检测中..." : "立即检测"}
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-8 px-2.5 text-xs"
                          onClick={() => {
                            setEditing(account);
                            setDialogOpen(true);
                          }}
                        >
                          <Pencil className="mr-1.5 h-3.5 w-3.5" />
                          编辑
                        </Button>
                        <Button size="sm" variant="ghost" className="h-8 px-2.5 text-xs" onClick={() => void handleDelete(account.id)}>
                          <Trash2 className="mr-1.5 h-3.5 w-3.5" />
                          删除
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <AccountFormDialog
        open={dialogOpen}
        onOpenChange={(open) => {
          setDialogOpen(open);
          if (!open) setEditing(null);
        }}
        initial={editing}
        onSubmit={handleSubmit}
        onValidate={handleValidate}
      />
      <AccountImportDialog open={importOpen} onOpenChange={setImportOpen} onImport={handleImport} />
    </div>
  );
}
