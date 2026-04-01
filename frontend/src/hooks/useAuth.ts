import { useState, useEffect } from 'react';

interface NostrUser {
  pubkey: string;
}

export const useAuth = () => {
  const [user, setUser] = useState<NostrUser | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const saved = localStorage.getItem('flspd_auth');
    if (saved) {
      setUser(JSON.parse(saved));
    }
    setLoading(false);
  }, []);

  const login = async () => {
    if (!(window as any).nostr) {
      alert('Nostr extension (NIP-07) not found. Please install Alby or similar.');
      return;
    }

    try {
      const pubkey = await (window as any).nostr.getPublicKey();
      const newUser = { pubkey };
      localStorage.setItem('flspd_auth', JSON.stringify(newUser));
      setUser(newUser);
    } catch (err) {
      console.error('Login failed', err);
    }
  };

  const logout = () => {
    localStorage.removeItem('flspd_auth');
    setUser(null);
  };

  const getAuthHeader = async (url: string, method: string) => {
    if (!user || !(window as any).nostr) return '';
    
    // NIP-98 HTTP Authentication
    const event = {
      kind: 27235,
      created_at: Math.floor(Date.now() / 1000),
      tags: [
        ['u', url],
        ['method', method],
      ],
      content: '',
    };

    const signedEvent = await (window as any).nostr.signEvent(event);
    const encodedEvent = btoa(JSON.stringify(signedEvent));
    return `Nostr ${encodedEvent}`;
  };

  return { user, loading, login, logout, getAuthHeader };
};
