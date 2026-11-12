import {
  createContext,
  type PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

export type FeedbackTone = "success" | "error" | "info";

type FeedbackMessage = {
  id: number;
  text: string;
  tone: FeedbackTone;
};

type FeedbackContextValue = {
  notify(text: string, tone?: FeedbackTone): void;
  dismiss(id: number): void;
};

const FeedbackContext = createContext<FeedbackContextValue | null>(null);
const messageLifetime = 5_000;

export function FeedbackProvider({ children }: PropsWithChildren) {
  const [messages, setMessages] = useState<FeedbackMessage[]>([]);
  const nextID = useRef(0);
  const timers = useRef(new Map<number, number>());

  const dismiss = useCallback((id: number) => {
    const timer = timers.current.get(id);
    if (timer) window.clearTimeout(timer);
    timers.current.delete(id);
    setMessages((current) => current.filter((message) => message.id !== id));
  }, []);

  const notify = useCallback(
    (text: string, tone: FeedbackTone = "info") => {
      const id = ++nextID.current;
      setMessages((current) => [...current.slice(-3), { id, text, tone }]);
      timers.current.set(
        id,
        window.setTimeout(() => dismiss(id), messageLifetime),
      );
    },
    [dismiss],
  );

  useEffect(
    () => () => {
      for (const timer of timers.current.values()) window.clearTimeout(timer);
      timers.current.clear();
    },
    [],
  );

  const value = useMemo(() => ({ notify, dismiss }), [dismiss, notify]);

  return (
    <FeedbackContext.Provider value={value}>
      {children}
      <aside className="feedback-viewport" aria-label="Notifications">
        {messages.map((message) => (
          <div
            className={`feedback-message feedback-message-${message.tone}`}
            role={message.tone === "error" ? "alert" : "status"}
            key={message.id}
          >
            <span className="feedback-symbol" aria-hidden="true">
              {message.tone === "success"
                ? "✓"
                : message.tone === "error"
                  ? "!"
                  : "i"}
            </span>
            <span>{message.text}</span>
            <button
              type="button"
              aria-label="Dismiss notification"
              onClick={() => dismiss(message.id)}
            >
              ×
            </button>
          </div>
        ))}
      </aside>
    </FeedbackContext.Provider>
  );
}

export function useFeedback() {
  return useContext(FeedbackContext);
}
