const API_BASE = '/api';
let accessToken: string | null = null;
let refreshPromise: Promise<boolean> | null = null;

export function setAccessToken(token: string | null) { accessToken = token; }

function getCookie(name: string): string | null {
  if (typeof document === 'undefined') return null;
  const value = document.cookie.split('; ').find((part) => part.startsWith(`${name}=`));
  return value ? decodeURIComponent(value.slice(name.length + 1)) : null;
}

function getAuthHeader(): Record<string, string> {
  return accessToken ? { Authorization: `Bearer ${accessToken}` } : {};
}

async function refreshAccessToken(): Promise<boolean> {
  if (refreshPromise) return refreshPromise;
  refreshPromise = (async () => {
  const csrf = getCookie('cp_csrf');
  if (!csrf) return false;
  const res = await fetch(`${API_BASE}/auth/refresh`, { method: 'POST', credentials: 'include', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf } });
  if (!res.ok) return false;
  const data = await res.json() as { accessToken: string };
  setAccessToken(data.accessToken);
  return true;
  })();
  try {
    return await refreshPromise;
  } finally {
    refreshPromise = null;
  }
}

async function request<T>(path: string, options: RequestInit = {}, canRefresh = true): Promise<T> {
  const csrf = getCookie('cp_csrf');
  const headers = {
    'Content-Type': 'application/json',
    ...getAuthHeader(),
    ...(csrf ? { 'X-CSRF-Token': csrf } : {}),
    ...options.headers,
  };

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
    credentials: 'include',
  });

  if (res.status === 401 && canRefresh && !path.startsWith('/auth/')) {
    if (await refreshAccessToken()) return request<T>(path, options, false);
    setAccessToken(null);
  }

  if (!res.ok) {
    let errMsg = `Request failed: ${res.statusText}`;
    try {
      const data = await res.json();
      if (data.error) errMsg = data.error;
    } catch {}
    throw new Error(errMsg);
  }

  if (res.status === 204) {
    return {} as T;
  }

  return res.json();
}

export const api = {
  get: <T>(path: string) => request<T>(path, { method: 'GET' }),
  post: <T>(path: string, body?: unknown) => request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }),
  put: <T>(path: string, body?: unknown) => request<T>(path, { method: 'PUT', body: body ? JSON.stringify(body) : undefined }),
  patch: <T>(path: string, body?: unknown) => request<T>(path, { method: 'PATCH', body: body ? JSON.stringify(body) : undefined }),
  delete: <T>(path: string, body?: unknown) => request<T>(path, {
    method: 'DELETE',
    body: body ? JSON.stringify(body) : undefined,
  }),
};
