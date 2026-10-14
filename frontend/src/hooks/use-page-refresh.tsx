import { createContext, useContext, useMemo, useState } from "react";

interface PageRefreshContextValue {
  refreshKey: number;
  triggerRefresh: () => void;
}

const PageRefreshContext = createContext<PageRefreshContextValue | null>(null);

export function PageRefreshProvider({ children }: { children: React.ReactNode }) {
  const [refreshKey, setRefreshKey] = useState(0);

  const value = useMemo<PageRefreshContextValue>(() => ({
    refreshKey,
    triggerRefresh: () => {
      setRefreshKey((current) => current + 1);
    },
  }), [refreshKey]);

  return <PageRefreshContext.Provider value={value}>{children}</PageRefreshContext.Provider>;
}

export function usePageRefresh() {
  const context = useContext(PageRefreshContext);
  if (!context) {
    throw new Error("usePageRefresh must be used within PageRefreshProvider");
  }
  return context;
}
