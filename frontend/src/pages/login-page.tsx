import { ChangeEvent, FormEvent, useState } from "react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ErrorAlert } from "@/components/ui/error-alert";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api";

export function LoginPage({ onSuccess }: { onSuccess: (username: string) => void }) {
  const [form, setForm] = useState({ username: "", password: "" });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const res = await api.login(form);
      onSuccess(res.username);
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : "登录失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-[#05080F] px-4 py-10 text-slate-100">
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(720px_380px_at_80%_-10%,rgba(34,211,238,0.16),transparent_60%),radial-gradient(640px_420px_at_-10%_110%,rgba(59,130,246,0.14),transparent_60%)]" />
      <Card className="relative w-full max-w-sm border-white/10 bg-white/[0.04] shadow-[0_24px_80px_rgba(0,0,0,0.5)] backdrop-blur-xl">
        <CardHeader className="mb-0 flex-col items-center justify-start gap-4 border-b border-white/10 pb-6 text-center">
          <span className="flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-cyan-400 to-blue-600 shadow-[0_0_32px_rgba(34,211,238,0.5)]">
            <img src="/favicon.png" alt="X" className="h-9 w-9 rounded-lg" />
          </span>
          <div className="space-y-2">
            <div className="text-[11px] font-medium uppercase tracking-[0.3em] text-cyan-300/80">
              Upstream X
            </div>
            <CardTitle className="text-2xl">管理员登录</CardTitle>
            <CardDescription>使用后台账号密码进入系统</CardDescription>
          </div>
        </CardHeader>
        <CardContent className="pt-6">
          <form className="space-y-5" onSubmit={handleSubmit}>
            <div className="space-y-2">
              <Label htmlFor="login-username">用户名</Label>
              <Input
                id="login-username"
                value={form.username}
                onChange={(e: ChangeEvent<HTMLInputElement>) => setForm((s) => ({ ...s, username: e.target.value }))}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="login-password">密码</Label>
              <Input
                id="login-password"
                type="password"
                value={form.password}
                onChange={(e: ChangeEvent<HTMLInputElement>) => setForm((s) => ({ ...s, password: e.target.value }))}
                required
              />
            </div>
            {error && <ErrorAlert title="登录失败" message={error} />}
            <Button className="w-full bg-gradient-to-r from-cyan-400 to-blue-500 font-semibold text-slate-950 shadow-[0_8px_30px_rgba(34,211,238,0.35)] hover:from-cyan-300 hover:to-blue-400" size="lg" disabled={loading}>
              {loading ? "登录中..." : "进入后台"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
