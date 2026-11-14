import { useCallback, useEffect, useRef, useState } from "react";

import { ApiError } from "../../shared/api/client";
import { type CatalogPage, fetchCatalogPage } from "./catalogApi";

type CatalogPageState<T> =
  | { status: "loading"; page: null; message: "" }
  | { status: "ready"; page: CatalogPage<T>; message: "" }
  | { status: "error"; page: null; message: string };

export function useCatalogPage<T>(path: string) {
  const controller = useRef<AbortController | null>(null);
  const [state, setState] = useState<CatalogPageState<T>>({
    status: "loading",
    page: null,
    message: "",
  });

  const load = useCallback(() => {
    controller.current?.abort();
    controller.current = new AbortController();
    const signal = controller.current.signal;
    setState({ status: "loading", page: null, message: "" });
    fetchCatalogPage<T>(path, signal)
      .then((page) => setState({ status: "ready", page, message: "" }))
      .catch((error: unknown) => {
        if (signal.aborted) return;
        setState({
          status: "error",
          page: null,
          message:
            error instanceof ApiError
              ? error.message
              : "Catalog is temporarily unavailable",
        });
      });
  }, [path]);

  useEffect(() => {
    load();
    return () => controller.current?.abort();
  }, [load]);

  return { ...state, retry: load };
}
