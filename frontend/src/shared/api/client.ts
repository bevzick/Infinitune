const defaultGatewayUrl = "http://localhost:8000";

export const gatewayUrl = (
  import.meta.env.VITE_GATEWAY_URL || defaultGatewayUrl
).replace(/\/$/, "");

type ApiErrorPayload = {
  error?: {
    code?: string;
    message?: string;
  };
};

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

export type ApiRequestOptions = RequestInit & {
  accessToken?: string;
};

export async function apiRequest<T>(
  path: string,
  options: ApiRequestOptions = {},
): Promise<T> {
  const response = await sendRequest(path, options);

  if (response.status === 204) {
    return undefined as T;
  }
  return (await readJson<T>(response)) as T;
}

export async function apiBlobRequest(
  path: string,
  options: ApiRequestOptions = {},
) {
  const response = await sendRequest(path, options);
  return response.blob();
}

async function sendRequest(path: string, options: ApiRequestOptions) {
  if (!path.startsWith("/") || path.startsWith("//")) {
    throw new Error("API path must be root-relative");
  }

  const { accessToken, ...init } = options;
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (accessToken) {
    headers.set("Authorization", `Bearer ${accessToken}`);
  }
  if (
    init.body &&
    !(init.body instanceof FormData) &&
    !headers.has("Content-Type")
  ) {
    headers.set("Content-Type", "application/json");
  }

  const response = await fetch(`${gatewayUrl}${path}`, {
    ...init,
    headers,
    credentials: "include",
  });

  if (!response.ok) {
    const payload = await readJson<ApiErrorPayload>(response);
    throw new ApiError(
      response.status,
      payload?.error?.code || "request_failed",
      payload?.error?.message ||
        `Request failed with status ${response.status}`,
    );
  }

  return response;
}

async function readJson<T>(response: Response): Promise<T | undefined> {
  if (!response.headers.get("Content-Type")?.includes("application/json")) {
    return undefined;
  }
  return (await response.json()) as T;
}
