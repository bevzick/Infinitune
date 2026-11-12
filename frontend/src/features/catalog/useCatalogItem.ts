import { useCallback, useEffect, useRef, useState } from "react";

import { ApiError } from "../../shared/api/client";
import { fetchCatalogItem } from "./catalogApi";

type CatalogResource = "artists" | "albums" | "tracks";

type CatalogItemState<T> =
  | { status: "idle"; item: null; message: "" }
  | { status: "loading"; item: null; message: "" }
  | { status: "ready"; item: T; message: "" }
  | { status: "error"; item: null; message: string };

export function useCatalogItem<T>(resource: CatalogResource, id?: string) {
  const controller = useRef<AbortController | null>(null);
  const [state, setState] = useState<CatalogItemState<T>>({
    status: id ? "loading" : "idle",
    item: null,
    message: "",
  });

  const load = useCallback(() => {
    controller.current?.abort();
    if (!id) {
      setState({ status: "idle", item: null, message: "" });
      return;
    }

    controller.current = new AbortController();
    const signal = controller.current.signal;
    setState({ status: "loading", item: null, message: "" });
    fetchCatalogItem<T>(resource, id, signal)
      .then((item) => setState({ status: "ready", item, message: "" }))
      .catch((error: unknown) => {
        if (signal.aborted) return;
        setState({
          status: "error",
          item: null,
          message:
            error instanceof ApiError
              ? error.message
              : "Catalog item is temporarily unavailable",
        });
      });
  }, [id, resource]);

  useEffect(() => {
    load();
    return () => controller.current?.abort();
  }, [load]);

  return { ...state, retry: load };
}
