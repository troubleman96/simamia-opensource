'use client';

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode
} from 'react';
import { SIMAMIA_API_BASE_URL } from '@/lib/simamia-api';

const TOKEN_KEY = 'simamia_local_auth_token';
const desktopLocalMode = process.env.NEXT_PUBLIC_SIMAMIA_DESKTOP === 'true';

export interface SimamiaUser {
  sub: string;
  name?: string;
  phone_number?: string;
}

export interface LocalAccountInput {
  name: string;
  phone: string;
  password: string;
  rememberMe: boolean;
}

export interface LocalLoginInput {
  phone: string;
  password: string;
  rememberMe: boolean;
}

interface AuthContextValue {
  user: SimamiaUser | null;
  authenticated: boolean;
  loading: boolean;
  getAccessToken: () => string | null;
  createAccount: (input: LocalAccountInput) => Promise<void>;
  signIn: (input: LocalLoginInput) => Promise<void>;
  logout: () => void;
}

interface APIResponse<T> {
  data?: T;
  error?: { message?: string };
}

interface SessionResponse {
  access_token?: string;
  user?: SimamiaUser;
}

const AuthContext = createContext<AuthContextValue | null>(null);

const desktopAuth: AuthContextValue = {
  user: { sub: 'local-preview', name: 'Simamia local workspace' },
  authenticated: true,
  loading: false,
  getAccessToken: () => 'desktop-local',
  createAccount: async () => {},
  signIn: async () => {},
  logout: () => {}
};

function readStoredToken() {
  return sessionStorage.getItem(TOKEN_KEY) || localStorage.getItem(TOKEN_KEY);
}

function persistToken(token: string, rememberMe: boolean) {
  sessionStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(TOKEN_KEY);
  (rememberMe ? localStorage : sessionStorage).setItem(TOKEN_KEY, token);
}

function clearStoredToken() {
  sessionStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(TOKEN_KEY);
}

async function requestAPI<T>(endpoint: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`${SIMAMIA_API_BASE_URL}${endpoint}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...init.headers
    }
  });
  const payload = (await response.json()) as APIResponse<T>;
  if (!response.ok || payload.data === undefined) {
    throw new Error(payload.error?.message || `Request failed (${response.status})`);
  }
  return payload.data;
}

export function LocalAuthRoot({ children }: { children: ReactNode }) {
  if (desktopLocalMode) {
    return <AuthContext.Provider value={desktopAuth}>{children}</AuthContext.Provider>;
  }
  return <BrowserLocalAuthProvider>{children}</BrowserLocalAuthProvider>;
}

function BrowserLocalAuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<SimamiaUser | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;
    const restoreSession = async () => {
      const savedToken = readStoredToken();
      if (!savedToken) {
        if (active) {
          setUser(null);
          setToken(null);
          setLoading(false);
        }
        return;
      }
      try {
        const data = await requestAPI<{ user: SimamiaUser }>('/auth/me', {
          headers: { Authorization: `Bearer ${savedToken}` },
          cache: 'no-store'
        });
        if (active) {
          setUser(data.user);
          setToken(savedToken);
        }
      } catch {
        clearStoredToken();
        if (active) {
          setUser(null);
          setToken(null);
        }
      } finally {
        if (active) setLoading(false);
      }
    };
    void restoreSession();
    const handleStorage = (event: StorageEvent) => {
      if (event.key === TOKEN_KEY) {
        setLoading(true);
        void restoreSession();
      }
    };
    window.addEventListener('storage', handleStorage);
    return () => {
      active = false;
      window.removeEventListener('storage', handleStorage);
    };
  }, []);

  const establishSession = useCallback(
    async (path: 'login' | 'register', input: LocalAccountInput | LocalLoginInput) => {
      const data = await requestAPI<SessionResponse>(`/auth/${path}`, {
        method: 'POST',
        body: JSON.stringify({
          ...(path === 'register' ? { name: (input as LocalAccountInput).name } : {}),
          phone: input.phone,
          password: input.password,
          remember_me: input.rememberMe
        })
      });
      if (!data.access_token || !data.user)
        throw new Error('The API returned an incomplete account session.');
      persistToken(data.access_token, input.rememberMe);
      setToken(data.access_token);
      setUser(data.user);
    },
    []
  );

  const createAccount = useCallback(
    (input: LocalAccountInput) => establishSession('register', input),
    [establishSession]
  );
  const signIn = useCallback(
    (input: LocalLoginInput) => establishSession('login', input),
    [establishSession]
  );
  const logout = useCallback(() => {
    const currentToken = token || readStoredToken();
    clearStoredToken();
    setToken(null);
    setUser(null);
    if (currentToken) {
      void fetch(`${SIMAMIA_API_BASE_URL}/auth/logout`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${currentToken}`
        },
        body: '{}'
      }).catch(() => {});
    }
  }, [token]);
  const getAccessToken = useCallback(() => token, [token]);
  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      authenticated: Boolean(user),
      loading,
      getAccessToken,
      createAccount,
      signIn,
      logout
    }),
    [user, loading, getAccessToken, createAccount, signIn, logout]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useSimamiaAuth() {
  const context = useContext(AuthContext);
  if (!context) throw new Error('useSimamiaAuth must be used within LocalAuthRoot');
  return context;
}
