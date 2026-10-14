import { ChangeEvent, FormEvent, useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ErrorAlert } from "@/components/ui/error-alert";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { type Account, type NewAPIAuthMode, type Sub2APIAuthMode } from "@/lib/types";

type FormState = {
  name: string;
  remark: string;
  type: "sub2api" | "newapi";
  base_url: string;
  sub2api_auth_mode: Sub2APIAuthMode;
  sub2api_login_body_template: string;
  newapi_auth_mode: NewAPIAuthMode;
  bearer_token: string;
  refresh_token: string;
  sub2api_email: string;
  sub2api_password: string;
  new_api_user: string;
  low_balance_threshold_usd: string;
  enabled: boolean;
};

const defaultState: FormState = {
  name: "",
  remark: "",
  type: "sub2api",
  base_url: "",
  sub2api_auth_mode: "refresh_token",
  sub2api_login_body_template: "",
  newapi_auth_mode: "manual",
  bearer_token: "",
  refresh_token: "",
  sub2api_email: "",
  sub2api_password: "",
  new_api_user: "",
  low_balance_threshold_usd: "0",
  enabled: true,
};

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initial?: Account | null;
  onSubmit: (payload: Record<string, unknown>) => Promise<void>;
  onValidate: (payload: Record<string, unknown>) => Promise<
    { access_token?: string; refresh_token?: string } | undefined
  >;
}

function FieldNote({ children }: { children: React.ReactNode }) {
  return <p className="text-xs leading-5 text-muted-foreground">{children}</p>;
}

export function AccountFormDialog({ open, onOpenChange, initial, onSubmit, onValidate }: Props) {
  const [form, setForm] = useState<FormState>(defaultState);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const isEditing = Boolean(initial);
  const isNewApi = form.type === "newapi";
  const isSub2API = form.type === "sub2api";
  const isSub2APIPasswordMode = form.type === "sub2api" && form.sub2api_auth_mode === "password";
  const isSub2APIRefreshTokenMode = form.type === "sub2api" && form.sub2api_auth_mode === "refresh_token";
  const isNewAPIManualMode = form.type === "newapi" && form.newapi_auth_mode === "manual";

  useEffect(() => {
    if (!initial) {
      setForm(defaultState);
      return;
    }
    setForm({
      name: initial.name,
      remark: initial.remark ?? "",
      type: initial.type,
      base_url: initial.base_url,
      sub2api_auth_mode: initial.sub2api_auth_mode ?? "refresh_token",
      sub2api_login_body_template: initial.sub2api_login_body_template ?? "",
      newapi_auth_mode: initial.newapi_auth_mode ?? "manual",
      bearer_token: "",
      refresh_token: "",
      sub2api_email: initial.sub2api_email ?? "",
      sub2api_password: "",
      new_api_user: initial.new_api_user ?? "",
      low_balance_threshold_usd: String(initial.low_balance_threshold_usd),
      enabled: initial.enabled,
    });
  }, [initial, open]);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const payload: Record<string, unknown> = {
        ...form,
        low_balance_threshold_usd: Number(form.low_balance_threshold_usd),
      };
      const validated = await onValidate(payload);
      // Validation may have rotated the Sub2API refresh token upstream;
      // persist the fresh credentials returned by validation, not the
      // burned ones still sitting in the form.
      if (validated?.refresh_token) {
        payload.refresh_token = validated.refresh_token;
      }
      if (validated?.access_token) {
        payload.bearer_token = validated.access_token;
      }
      await onSubmit(payload);
      onOpenChange(false);
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : "提交失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[calc(100vh-2rem)] overflow-hidden p-0 sm:max-w-3xl">
        <DialogHeader className="border-b px-5 py-4 sm:px-6">
          <DialogTitle>{isEditing ? "编辑上游账号" : "新增上游账号"}</DialogTitle>
          <DialogDescription>保存前会先即时校验接口可用性。</DialogDescription>
        </DialogHeader>
        <form className="flex max-h-[calc(100vh-10rem)] flex-col" onSubmit={handleSubmit}>
          <div className="flex-1 space-y-5 overflow-y-auto px-5 py-4 sm:px-6">
            <section className="space-y-4">
              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="account-name">账号名称</Label>
                  <Input
                    id="account-name"
                    placeholder="例如：主站 New API"
                    value={form.name}
                    onChange={(e: ChangeEvent<HTMLInputElement>) => setForm((s) => ({ ...s, name: e.target.value }))}
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="account-type">类型</Label>
                  <Select value={form.type} onValueChange={(value: "sub2api" | "newapi") => setForm((s) => ({ ...s, type: value }))}>
                    <SelectTrigger id="account-type">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="sub2api">Sub2API</SelectItem>
                      <SelectItem value="newapi">New API</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>

              <div className="space-y-2">
                <Label htmlFor="account-remark">备注</Label>
                <Textarea
                  id="account-remark"
                  placeholder="选填，可记录站点用途、账号归属、特殊说明"
                  value={form.remark}
                  onChange={(e: ChangeEvent<HTMLTextAreaElement>) => setForm((s) => ({ ...s, remark: e.target.value }))}
                  rows={3}
                  className="resize-none"
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="account-base-url">Base URL</Label>
                <Input
                  id="account-base-url"
                  className="font-mono text-[13px]"
                  placeholder="https://example.com"
                  value={form.base_url}
                  onChange={(e: ChangeEvent<HTMLInputElement>) => setForm((s) => ({ ...s, base_url: e.target.value }))}
                  required
                />
                <FieldNote>填写上游根地址即可，系统会自动拼接固定接口路径。</FieldNote>
              </div>
            </section>

            <section className="space-y-4 border-t pt-5">
              {!isNewApi && (
                <div className="space-y-2">
                  <Label htmlFor="sub2api-auth-mode">Sub2API 认证方式</Label>
                  <Select
                    value={form.sub2api_auth_mode}
                    onValueChange={(value: Sub2APIAuthMode) => setForm((s) => ({ ...s, sub2api_auth_mode: value }))}
                  >
                    <SelectTrigger id="sub2api-auth-mode">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="refresh_token">Refresh Token</SelectItem>
                      <SelectItem value="password">账号密码</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              )}
              <div className={`grid gap-4 ${isNewAPIManualMode || isSub2APIPasswordMode ? "md:grid-cols-2" : ""}`}>
                {isSub2API && isSub2APIPasswordMode && (
                  <>
                    <div className="space-y-2">
                      <Label htmlFor="sub2api-email">登录邮箱</Label>
                      <Input
                        id="sub2api-email"
                        type="email"
                        placeholder={isEditing ? "留空则保持原值" : "请输入 Sub2API 登录邮箱"}
                        value={form.sub2api_email}
                        onChange={(e: ChangeEvent<HTMLInputElement>) => setForm((s) => ({ ...s, sub2api_email: e.target.value }))}
                        required={isSub2APIPasswordMode && !isEditing}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="sub2api-password">登录密码</Label>
                      <Input
                        id="sub2api-password"
                        type="password"
                        placeholder={isEditing ? "留空则保持原值" : "请输入 Sub2API 登录密码"}
                        value={form.sub2api_password}
                        onChange={(e: ChangeEvent<HTMLInputElement>) => setForm((s) => ({ ...s, sub2api_password: e.target.value }))}
                        required={!isEditing}
                      />
                    </div>
                  </>
                )}
                {isSub2API && isSub2APIRefreshTokenMode && (
                  <div className="space-y-2">
                    <Label htmlFor="account-refresh-token">Refresh Token</Label>
                    <Input
                      id="account-refresh-token"
                      type="password"
                      placeholder={isEditing ? "留空则保持原值" : "请输入 Sub2API Refresh Token"}
                      value={form.refresh_token}
                      onChange={(e: ChangeEvent<HTMLInputElement>) => setForm((s) => ({ ...s, refresh_token: e.target.value }))}
                      required={!isEditing}
                    />
                  </div>
                )}
                {isNewAPIManualMode && (
                  <>
                    <div className="space-y-2">
                      <Label htmlFor="account-bearer-token">Bearer Token</Label>
                      <Input
                        id="account-bearer-token"
                        type="password"
                        placeholder={isEditing ? "留空则保持原值" : "请输入 Bearer Token"}
                        value={form.bearer_token}
                        onChange={(e: ChangeEvent<HTMLInputElement>) => setForm((s) => ({ ...s, bearer_token: e.target.value }))}
                        required={!isEditing}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="account-new-api-user">New-Api-User</Label>
                      <Input
                        id="account-new-api-user"
                        placeholder={isEditing ? "留空则保持原值" : "请输入 New-Api-User"}
                        value={form.new_api_user}
                        onChange={(e: ChangeEvent<HTMLInputElement>) => setForm((s) => ({ ...s, new_api_user: e.target.value }))}
                        required={!isEditing}
                      />
                    </div>
                  </>
                )}
              </div>
              {isSub2API && isSub2APIPasswordMode && (
                <div className="space-y-2">
                  <Label htmlFor="sub2api-login-body-template">自定义登录请求体</Label>
                  <Textarea
                    id="sub2api-login-body-template"
                    rows={6}
                    className="font-mono text-[12px] leading-5"
                    placeholder={`留空则使用默认请求体\n{\n  "email": "{{email}}",\n  "password": "{{password}}"\n}`}
                    value={form.sub2api_login_body_template}
                    onChange={(e: ChangeEvent<HTMLTextAreaElement>) =>
                      setForm((s) => ({ ...s, sub2api_login_body_template: e.target.value }))
                    }
                  />
                  <FieldNote>
                    某些站点改过登录接口时可在这里填写 JSON 请求体，推荐使用 <code>{"{{email}}"}</code> 和
                    <code>{"{{password}}"}</code>。留空则默认使用邮箱和密码字段。
                  </FieldNote>
                </div>
              )}
              <FieldNote>
                {isNewApi
                  ? "New API 需要同时填写 Bearer Token 和 New-Api-User。"
                  : isSub2APIPasswordMode
                    ? "账号密码模式下，系统会自动登录并维护一套独立 token，不受 refresh token 轮换影响（推荐）。如果上游开启了人机验证（Turnstile / 腾讯 / 阿里云）或 2FA，服务端无法自动登录，请改用 Refresh Token 模式。"
                    : "新版 Sub2API 的 refresh token（rt_ 开头）是一次性轮换的：每次校验/检测都会换新，校验成功后系统会自动保存最新 token。不要把浏览器正在使用的会话 token 填进来（会被轮换或会话绑定撤销），建议在上游单独登录一个专用会话取 token 后立即填入；长期使用更推荐账号密码模式。"}
              </FieldNote>
            </section>

            <section className="grid gap-4 border-t pt-5 md:grid-cols-[minmax(0,1fr)_260px] md:items-end">
              <div className="space-y-2">
                <Label htmlFor="account-threshold">低余额阈值（$）</Label>
                <Input
                  id="account-threshold"
                  type="number"
                  step="0.0001"
                  min="0"
                  value={form.low_balance_threshold_usd}
                  onChange={(e: ChangeEvent<HTMLInputElement>) => setForm((s) => ({ ...s, low_balance_threshold_usd: e.target.value }))}
                  required
                />
                <FieldNote>余额低于该值时触发低余额告警。</FieldNote>
              </div>

              <div className="flex items-center justify-between rounded-md border bg-muted/20 px-4 py-3">
                <div className="pr-4">
                  <Label htmlFor="account-enabled">启用定时检测</Label>
                  <FieldNote>关闭后仅可手动检测。</FieldNote>
                </div>
                <Switch
                  id="account-enabled"
                  checked={form.enabled}
                  onCheckedChange={(checked) => setForm((s) => ({ ...s, enabled: checked }))}
                />
              </div>
            </section>

            {error && <ErrorAlert title="校验或保存失败" message={error} />}
          </div>

          <div className="flex shrink-0 items-center justify-end gap-3 border-t px-5 py-4 sm:px-6">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              取消
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting ? "校验并保存中..." : "校验并保存"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
