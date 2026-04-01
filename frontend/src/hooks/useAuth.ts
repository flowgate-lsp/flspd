import { useState, useEffect } from 'react';
import { nip19, finalizeEvent, getPublicKey } from 'nostr-tools';

interface NostrUser {
  pubkey: string;
  nsec: string;
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

  const login = async (nsec: string) => {
    try {
      const { type, data: privkey } = nip19.decode(nsec);
      if (type !== 'nsec') throw new Error('Not an nsec');
      
      const pubkey = getPublicKey(privkey as Uint8Array);
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
    if (!user || !user.nsec) return '';
    
    try {
      const { data: privkey } = nip19.decode(user.nsec);
      
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

      const signedEvent = finalizeEvent(event, privkey as Uint8Array);
      const encodedEvent = btoa(JSON.stringify(signedEvent));
      return `Nostr ${encodedEvent}`;
    } catch (err) {
      console.error('Failed to get auth header', err);
      return '';
    }
  };

  return { user, loading, login, logout, getAuthHeader };
};
