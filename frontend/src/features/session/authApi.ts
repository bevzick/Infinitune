import { ApiError, apiRequest } from "../../shared/api/client";

const csrfCookieName = "infinitune_csrf";

export type Identity = {
  userId: string;
  roles: string[];
  issuedAt: string;
  expiresAt: string;
};

export type AccessSession = {
  accessToken: string;
  expiresAt: string;
  identity: Identity;
};

type TokenResponse = {
  access_token: string;
  token_type: string;
  expires_at: string;
};

type IdentityResponse = {
  user_id: string;
  roles: string[];
  issued_at: string;
  expires_at: string;
};

type RegistrationResponse = {
  id: string;
  email: string;
  status: string;
  email_verified: boolean;
  created_at: string;
};

export async function register(email: string, password: string) {
  return apiRequest<RegistrationResponse>("/v1/auth/register", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
}

export async function login(
  email: string,
  password: string,
): Promise<AccessSession> {
  const tokens = await apiRequest<TokenResponse>("/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  return completeSession(tokens);
}

let refreshPromise: Promise<AccessSession | null> | null = null;

export function refresh(): Promise<AccessSession | null> {
  if (refreshPromise) {
    return refreshPromise;
  }
  refreshPromise = performRefresh().finally(() => {
    refreshPromise = null;
  });
  return refreshPromise;
}

async function performRefresh(): Promise<AccessSession | null> {
  const csrfToken = readCookie(csrfCookieName);
  if (!csrfToken) {
    return null;
  }
  try {
    const tokens = await apiRequest<TokenResponse>("/v1/auth/refresh", {
      method: "POST",
      headers: { "X-CSRF-Token": csrfToken },
    });
    return completeSession(tokens);
  } catch (error) {
    if (
      error instanceof ApiError &&
      (error.status === 401 || error.status === 403)
    ) {
      return null;
    }
    throw error;
  }
}

export async function logout(): Promise<void> {
  const csrfToken = readCookie(csrfCookieName);
  await apiRequest<void>("/v1/auth/logout", {
    method: "POST",
    headers: csrfToken ? { "X-CSRF-Token": csrfToken } : undefined,
  });
}

async function completeSession(tokens: TokenResponse): Promise<AccessSession> {
  if (
    tokens.token_type.toLowerCase() !== "bearer" ||
    !tokens.access_token ||
    !Number.isFinite(Date.parse(tokens.expires_at))
  ) {
    throw new ApiError(
      502,
      "invalid_auth_response",
      "Authentication response is invalid",
    );
  }
  const response = await apiRequest<IdentityResponse>("/v1/auth/identity", {
    accessToken: tokens.access_token,
  });
  return {
    accessToken: tokens.access_token,
    expiresAt: tokens.expires_at,
    identity: {
      userId: response.user_id,
      roles: response.roles,
      issuedAt: response.issued_at,
      expiresAt: response.expires_at,
    },
  };
}

function readCookie(name: string): string | null {
  const prefix = `${encodeURIComponent(name)}=`;
  for (const part of document.cookie.split(";")) {
    const cookie = part.trim();
    if (cookie.startsWith(prefix)) {
      return decodeURIComponent(cookie.slice(prefix.length));
    }
  }
  return null;
}
