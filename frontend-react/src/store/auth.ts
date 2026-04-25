import { create } from 'zustand';
import type { User } from '../types/api';

interface AuthState {
  token: string | null;
  user: User | null;
  setAuth: (token: string | null, user: User | null) => void;
  logout: () => void;
}

const tokenKey = 'eino.access_token';
const userKey = 'eino.user';

const readUser = () => {
  const raw = localStorage.getItem(userKey);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as User;
  } catch {
    return null;
  }
};

export const useAuthStore = create<AuthState>((set) => ({
  token: localStorage.getItem(tokenKey),
  user: readUser(),
  setAuth: (token, user) => {
    if (token) localStorage.setItem(tokenKey, token);
    else localStorage.removeItem(tokenKey);
    if (user) localStorage.setItem(userKey, JSON.stringify(user));
    else localStorage.removeItem(userKey);
    set({ token, user });
  },
  logout: () => {
    localStorage.removeItem(tokenKey);
    localStorage.removeItem(userKey);
    set({ token: null, user: null });
  },
}));
