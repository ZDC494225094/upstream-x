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
    <div className="flex min-h-screen items-center justify-center bg-muted/20 px-4 py-10">
      <Card className="w-full max-w-sm border bg-card shadow-lg">
        <CardHeader className="mb-0 flex-col items-center justify-start gap-4 border-b pb-6 text-center">
          <img src="/favicon.png" alt="X" className="h-12 w-12 rounded-xl border bg-background p-1.5 shadow-sm" />
          <div className="space-y-2">
            <div className="text-[11px] font-medium uppercase tracking-[0.22em] text-muted-foreground">
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
            <Button className="w-full" size="lg" disabled={loading}>
              {loading ? "登录中..." : "进入后台"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
