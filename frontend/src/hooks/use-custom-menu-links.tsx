import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";

import { api } from "@/lib/api";
import type { CustomMenuLink } from "@/lib/types";

interface CustomMenuLinksContextValue {
  links: CustomMenuLink[];
  loading: boolean;
  reload: () => Promise<void>;
  setLinks: (links: CustomMenuLink[]) => void;
}

const CustomMenuLinksContext = createContext<CustomMenuLinksContextValue | null>(null);

export function CustomMenuLinksProvider({ children }: { children: React.ReactNode }) {
  const [links, setLinks] = useState<CustomMenuLink[]>([]);
  const [loading, setLoading] = useState(true);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const result = await api.getCustomMenuLinks();
      setLinks(result.items ?? []);
    } catch {
      setLinks([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, []);

  const value = useMemo<CustomMenuLinksContextValue>(() => ({
    links,
    loading,
    reload,
    setLinks,
  }), [links, loading, reload]);

  return <CustomMenuLinksContext.Provider value={value}>{children}</CustomMenuLinksContext.Provider>;
}

export function useCustomMenuLinks() {
  const context = useContext(CustomMenuLinksContext);
  if (!context) {
    throw new Error("useCustomMenuLinks must be used within CustomMenuLinksProvider");
  }
  return context;
}
