import { ChangeEvent, useState } from "react";
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
import type { SettingsSnapshot } from "@/lib/types";

interface SettingsImportDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onImport: (payload: unknown) => Promise<SettingsSnapshot>;
}

export function SettingsImportDialog({ open, onOpenChange, onImport }: SettingsImportDialogProps) {
  const [fileName, setFileName] = useState("");
  const [payload, setPayload] = useState<unknown>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  function resetState() {
    setFileName("");
    setPayload(null);
    setError(null);
    setSubmitting(false);
  }

  async function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    setError(null);
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
      await onImport(payload);
      onOpenChange(false);
      resetState();
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
          <DialogTitle>导入系统设置 JSON</DialogTitle>
          <DialogDescription>导入后会覆盖当前调度、日志保留、SMTP 和自定义菜单设置，并立即热更新调度器。</DialogDescription>
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

          {error && <ErrorAlert title="导入系统设置失败" message={error} />}

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
