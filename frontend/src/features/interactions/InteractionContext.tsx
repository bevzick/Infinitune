import {
  createContext,
  type PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

import { useSession } from "../session/SessionContext";
import {
  type InteractionKind,
  listInteractionIDs,
  setInteraction,
} from "./interactionApi";

export const interactionsChangedEvent = "infinitune:interactions-changed";

type SavedState = Record<InteractionKind, Set<string>>;

type InteractionContextValue = {
  available: boolean;
  loaded: boolean;
  isActive(kind: InteractionKind, id: string): boolean;
  isBusy(kind: InteractionKind, id: string): boolean;
  toggle(kind: InteractionKind, id: string): Promise<void>;
};

const InteractionContext = createContext<InteractionContextValue | null>(null);

export function InteractionProvider({ children }: PropsWithChildren) {
  const { accessToken } = useSession();
  const [saved, setSaved] = useState<SavedState>(emptyState);
  const [busy, setBusy] = useState<Set<string>>(new Set());
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    if (!accessToken) {
      setSaved(emptyState());
      setBusy(new Set());
      setLoaded(false);
      return;
    }
    const controller = new AbortController();
    setLoaded(false);
    Promise.all(
      (["track", "album", "artist"] as const).map(
        async (kind) =>
          [
            kind,
            await listInteractionIDs(accessToken, kind, controller.signal),
          ] as const,
      ),
    )
      .then((entries) => {
        setSaved(
          Object.fromEntries(
            entries.map(([kind, ids]) => [kind, new Set(ids)]),
          ) as SavedState,
        );
        setLoaded(true);
      })
      .catch(() => {
        if (!controller.signal.aborted) setLoaded(true);
      });
    return () => controller.abort();
  }, [accessToken]);

  const toggle = useCallback(
    async (kind: InteractionKind, id: string) => {
      if (!accessToken) return;
      const key = interactionKey(kind, id);
      if (busy.has(key)) return;
      const wasActive = saved[kind].has(id);

      setBusy((current) => new Set(current).add(key));
      setSaved((current) => updateSaved(current, kind, id, !wasActive));
      try {
        await setInteraction(accessToken, kind, id, !wasActive);
        window.dispatchEvent(new Event(interactionsChangedEvent));
      } catch (error) {
        setSaved((current) => updateSaved(current, kind, id, wasActive));
        throw error;
      } finally {
        setBusy((current) => {
          const next = new Set(current);
          next.delete(key);
          return next;
        });
      }
    },
    [accessToken, busy, saved],
  );

  const value = useMemo<InteractionContextValue>(
    () => ({
      available: Boolean(accessToken),
      loaded,
      isActive: (kind, id) => saved[kind].has(id),
      isBusy: (kind, id) => busy.has(interactionKey(kind, id)),
      toggle,
    }),
    [accessToken, busy, loaded, saved, toggle],
  );

  return (
    <InteractionContext.Provider value={value}>
      {children}
    </InteractionContext.Provider>
  );
}

export function useInteractions() {
  return useContext(InteractionContext);
}

function emptyState(): SavedState {
  return { track: new Set(), album: new Set(), artist: new Set() };
}

function interactionKey(kind: InteractionKind, id: string) {
  return `${kind}:${id}`;
}

function updateSaved(
  current: SavedState,
  kind: InteractionKind,
  id: string,
  active: boolean,
) {
  const next = { ...current, [kind]: new Set(current[kind]) };
  if (active) next[kind].add(id);
  else next[kind].delete(id);
  return next;
}
