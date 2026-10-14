import { ChangeEvent, useMemo, useState } from "react";
import { Upload } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ErrorAlert } from "@/components/ui/error-alert";
import { FilePicker } from "@/components/ui/file-picker";
import type { AccountImportResult } from "@/lib/types";

interface AccountImportDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onImport: (payload: unknown) => Promise<AccountImportResult>;
}

export function AccountImportDialog({ open, onOpenChange, onImport }: AccountImportDialogProps) {
  const [fileName, setFileName] = useState("");
  const [payload, setPayload] = useState<unknown>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<AccountImportResult | null>(null);

  const summaryText = useMemo(() => {
    if (!result) return "";
    return `新增 ${result.created} 个，更新 ${result.updated} 个，失败 ${result.failed} 个`;
  }, [result]);

  function resetState() {
    setFileName("");
    setPayload(null);
    setError(null);
    setSubmitting(false);
    setResult(null);
  }

  async function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    setError(null);
    setResult(null);
    setPayload(null);
    setFileName(file?.name ?? "");

    if (!file) return;

    try {
      const text = await file.text();
      const parsed = JSON.parse(text) as unknown;
      setPayload(parsed);
    } catch {
      setError("JSON 文件解析失败，请检查格式。");
    }
  }

  async function handleSubmit() {
    if (!payload) {
      setError("请先选择一个 JSON 文件。");
      return;
    }

    setSubmitting(true);
    setError(null);
    try {
      const importResult = await onImport(payload);
      setResult(importResult);
    } catch (importError) {
      setError(importError instanceof Error ? importError.message : "导入失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        onOpenChange(nextOpen);
        if (!nextOpen) {
          resetState();
        }
      }}
    >
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>导入账号 JSON</DialogTitle>
          <DialogDescription>支持导入系统导出的 JSON，或直接导入账号数组。若账号 ID 或同名同类型已存在，则自动覆盖更新。</DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="rounded-lg border border-dashed bg-muted/20 p-4">
            <div className="mb-3 text-sm font-medium">选择导入文件</div>
            <FilePicker
              accept="application/json,.json"
              fileName={fileName}
              onChange={handleFileChange}
              placeholder="请选择一个 .json 文件"
            />
          </div>

          {error && <ErrorAlert title="导入账号失败" message={error} />}

          {result && (
            <div className="space-y-3 rounded-lg border bg-muted/20 p-4">
              <div className="text-sm font-medium">{summaryText}</div>
              {result.errors.length > 0 && (
                <div className="max-h-52 space-y-2 overflow-y-auto pr-1 text-sm [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
                  {result.errors.map((item) => (
                    <div key={`${item.index}-${item.name}`} className="rounded-md border bg-background px-3 py-2">
                      <div className="font-medium">{item.name || `第 ${item.index + 1} 项`}</div>
                      <div className="mt-1 text-xs text-muted-foreground">{item.message}</div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          <div className="flex items-center justify-end gap-3">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              关闭
            </Button>
            <Button type="button" onClick={() => void handleSubmit()} disabled={submitting || !payload}>
              <Upload className="mr-2 h-4 w-4" />
              {submitting ? "导入中..." : "开始导入"}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
