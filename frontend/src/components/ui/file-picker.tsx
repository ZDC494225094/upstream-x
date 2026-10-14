import { FileJson2, Upload } from "lucide-react";
import { type ChangeEvent, useId } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

interface FilePickerProps {
  accept?: string;
  className?: string;
  fileName?: string;
  onChange: (event: ChangeEvent<HTMLInputElement>) => void;
  placeholder?: string;
}

export function FilePicker({
  accept,
  className,
  fileName,
  onChange,
  placeholder = "请选择文件",
}: FilePickerProps) {
  const inputID = useId();

  return (
    <div className={cn("rounded-lg border bg-background p-3", className)}>
      <Input id={inputID} type="file" accept={accept} className="sr-only" onChange={onChange} />
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <label htmlFor={inputID}>
          <Button type="button" variant="outline" className="w-full cursor-pointer sm:w-auto" asChild>
            <span>
              <Upload className="mr-2 h-4 w-4" />
              选择文件
            </span>
          </Button>
        </label>
        <div className="min-w-0 flex-1 rounded-md border bg-muted/30 px-3 py-2">
          <div className="flex items-center gap-2">
            <FileJson2 className="h-4 w-4 shrink-0 text-muted-foreground" />
            <span className="truncate text-sm font-medium">{fileName || placeholder}</span>
          </div>
        </div>
      </div>
    </div>
  );
}
