import { writable } from 'svelte/store';
import { api, setAccessToken } from '$lib/api/client';
import type { User } from '@cpbridge/contracts';

export interface AuthState {
  user: User | null;
  token: string | null;
  loading: boolean;
}

function createAuthStore() {
  const { subscribe, set } = writable<AuthState>({
    user: null,
    token: null,
    loading: true,
  });

  return {
    subscribe,
    init: async () => {
      if (typeof window === 'undefined') return;
      try {
        const refreshed = await api.post<{ accessToken: string }>('/auth/refresh');
        setAccessToken(refreshed.accessToken);
        const res = await api.get<{ user: User }>('/auth/me');
        set({ user: res.user, token: refreshed.accessToken, loading: false });
      } catch {
        setAccessToken(null);
        set({ user: null, token: null, loading: false });
      }
    },
    setAuth: (user: User, token: string) => {
      setAccessToken(token);
      set({ user, token, loading: false });
    },
    logout: () => {
      if (typeof window !== 'undefined') {
        void api.post('/auth/logout').catch(() => undefined);
      }
      setAccessToken(null);
      set({ user: null, token: null, loading: false });
    },
    logoutAll: async () => {
      await api.post('/auth/logout-all');
      setAccessToken(null);
      set({ user: null, token: null, loading: false });
    },
  };
}

export const auth = createAuthStore();
