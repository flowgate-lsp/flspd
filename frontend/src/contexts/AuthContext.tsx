import React, { createContext, useContext, useState, useEffect } from 'react';
import { nip19, finalizeEvent, getPublicKey } from 'nostr-tools';

export interface NostrUser {
  pubkey: string;
  nsec: string;
}

interface AuthContextType {
  user: NostrUser | null;
  loading: boolean;
  login: (nsec: string) => Promise<void>;
  logout: () => void;
  getAuthHeader: (url: string, method: string) => Promise<string>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<NostrUser | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const saved = localStorage.getItem('flspd_auth');
    if (saved) {
      setUser(JSON.parse(saved));
    }
    setLoading(false);

    const handleUnauthorized = () => {
      logout();
    };

    window.addEventListener('flspd_unauthorized', handleUnauthorized);
    return () => window.removeEventListener('flspd_unauthorized', handleUnauthorized);
  }, []);

  const login = async (nsec: string) => {
    try {
      const { type, data: privkey } = nip19.decode(nsec);
      if (type !== 'nsec') throw new Error('Not an nsec');
      
      const pubkey = getPublicKey(privkey as Uint8Array);
      
      const testUrl = `${window.location.origin}/dashboard`;
      const event = {
        kind: 27235,
        created_at: Math.floor(Date.now() / 1000),
        tags: [
          ['u', testUrl],
          ['m', 'POST'],
        ],
        content: '',
      };
      const signedEvent = finalizeEvent(event, privkey as Uint8Array);
      const encodedEvent = btoa(JSON.stringify(signedEvent));

      const resp = await fetch(testUrl, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Nostr ${encodedEvent}`,
        },
        body: JSON.stringify({
          jsonrpc: '2.0',
          method: 'get_summary',
          params: {},
          id: 'login-check',
        }),
      });

      if (!resp.ok) {
        if (resp.status === 403) throw new Error('Unauthorized: Key is not listed as an admin.');
        throw new Error(`Server returned ${resp.status}`);
      }

      const newUser = { pubkey, nsec };
      localStorage.setItem('flspd_auth', JSON.stringify(newUser));
      setUser(newUser);
    } catch (err) {
      console.error('Login failed', err);
      throw err;
    }
  };

  const logout = () => {
    localStorage.removeItem('flspd_auth');
    setUser(null);
  };

  const getAuthHeader = async (url: string, method: string) => {
    const u = user;
    if (!u || !u.nsec) return '';
    
    try {
      const { data: privkey } = nip19.decode(u.nsec);
      const event = {
        kind: 27235,
        created_at: Math.floor(Date.now() / 1000),
        tags: [
          ['u', url],
          ['m', method],
        ],
        content: '',
      };

      const signedEvent = finalizeEvent(event, privkey as Uint8Array);
      const encodedEvent = btoa(JSON.stringify(signedEvent));
      return `Nostr ${encodedEvent}`;
    } catch (err) {
      console.error('Failed to get auth header', err);
      return '';
    }
  };

  return (
    <AuthContext.Provider value={{ user, loading, login, logout, getAuthHeader }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};
