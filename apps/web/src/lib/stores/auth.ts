import { writable } from 'svelte/store';
import { api } from '$lib/api/client';
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
      const token = localStorage.getItem('cp_token');
      if (!token) {
        set({ user: null, token: null, loading: false });
        return;
      }

      try {
        const res = await api.get<{ user: User }>('/auth/me');
        set({ user: res.user, token, loading: false });
      } catch {
        localStorage.removeItem('cp_token');
        set({ user: null, token: null, loading: false });
      }
    },
    setAuth: (user: User, token: string) => {
      if (typeof window !== 'undefined') {
        localStorage.setItem('cp_token', token);
      }
      set({ user, token, loading: false });
    },
    logout: () => {
      if (typeof window !== 'undefined') {
        localStorage.removeItem('cp_token');
      }
      set({ user: null, token: null, loading: false });
    },
  };
}

export const auth = createAuthStore();
