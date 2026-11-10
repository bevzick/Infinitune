import {
  createContext,
  type PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

import {
  type AccessSession,
  type Identity,
  login as loginRequest,
  logout as logoutRequest,
  refresh,
} from "./authApi";

type SessionStatus = "loading" | "authenticated" | "anonymous" | "error";

type SessionContextValue = {
  status: SessionStatus;
  identity: Identity | null;
  accessToken: string | null;
  login(email: string, password: string): Promise<void>;
  logout(): Promise<void>;
  retry(): void;
};

const SessionContext = createContext<SessionContextValue | null>(null);
const refreshLeadTime = 30_000;

export function SessionProvider({ children }: PropsWithChildren) {
  const [status, setStatus] = useState<SessionStatus>("loading");
  const [session, setSession] = useState<AccessSession | null>(null);
  const [restoreAttempt, setRestoreAttempt] = useState(0);

  const setCurrentSession = useCallback((next: AccessSession | null) => {
    setSession(next);
    setStatus(next ? "authenticated" : "anonymous");
  }, []);

  useEffect(() => {
    let active = true;
    if (restoreAttempt > 0) setStatus("loading");
    refresh()
      .then((next) => {
        if (active) setCurrentSession(next);
      })
      .catch(() => {
        if (active) {
          setSession(null);
          setStatus("error");
        }
      });
    return () => {
      active = false;
    };
  }, [restoreAttempt, setCurrentSession]);

  useEffect(() => {
    if (!session) return;
    const expiresAt = Date.parse(session.expiresAt);
    const delay = Math.max(0, expiresAt - Date.now() - refreshLeadTime);
    const timer = window.setTimeout(
      () => {
        refresh()
          .then(setCurrentSession)
          .catch(() => {
            setSession(null);
            setStatus("error");
          });
      },
      Math.min(delay, 2_147_000_000),
    );
    return () => window.clearTimeout(timer);
  }, [session, setCurrentSession]);

  const login = useCallback(
    async (email: string, password: string) => {
      setCurrentSession(await loginRequest(email, password));
    },
    [setCurrentSession],
  );

  const logout = useCallback(async () => {
    try {
      await logoutRequest();
    } finally {
      setCurrentSession(null);
    }
  }, [setCurrentSession]);

  const retry = useCallback(() => {
    setStatus("loading");
    setRestoreAttempt((attempt) => attempt + 1);
  }, []);

  const value = useMemo<SessionContextValue>(
    () => ({
      status,
      identity: session?.identity ?? null,
      accessToken: session?.accessToken ?? null,
      login,
      logout,
      retry,
    }),
    [status, session, login, logout, retry],
  );

  return (
    <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
  );
}

export function useSession() {
  const value = useContext(SessionContext);
  if (!value) throw new Error("useSession must be used inside SessionProvider");
  return value;
}
