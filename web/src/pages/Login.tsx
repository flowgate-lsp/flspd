import React, { useState } from 'react';
import { GlassCard } from '../components/GlassCard';
import { Activity, ShieldCheck, Key, Lock, AlertTriangle } from 'lucide-react';
import { useAuth } from '../hooks/useAuth';
import { useSystemInfo } from '../hooks/useSystemInfo';

export const Login: React.FC = () => {
  const { login } = useAuth();
  const { info, loading: infoLoading } = useSystemInfo();
  const [nsec, setNsec] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  // Don't offer a login form that's guaranteed to fail: no admin pubkey
  // configured means the backend rejects every auth attempt with 403
  // regardless of key. Still let the initial /info fetch resolve first so
  // this doesn't flash on a healthy backend.
  const dashboardDisabled = !infoLoading && info !== null && !info.dashboard_enabled;

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!nsec.startsWith('nsec1')) {
      setError('Please enter a valid nsec (starting with nsec1)');
      return;
    }

    setLoading(true);
    setError('');
    try {
      await login(nsec);
    } catch (err: any) {
      setError(err.message || 'Authentication failed. Please check your key.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-6 transition-all duration-700">
      {/* Background kinetic effect */}
      <div className="fixed inset-0 overflow-hidden pointer-events-none">
        <div className="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary/10 blur-[120px] rounded-full animate-pulse" />
        <div className="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-secondary/10 blur-[120px] rounded-full animate-pulse delay-700" />
      </div>

      <GlassCard className="w-full max-w-md p-10 text-center relative z-10 border border-outline hover:border-primary/30 transition-colors duration-500">
        <div className="flex justify-center mb-8">
           <div className="w-20 h-20 rounded-2xl bg-primary/10 flex items-center justify-center animate-glow-pulse border border-primary/20 shadow-[0_0_40px_rgba(0,186,255,0.2)]">
             <Activity className="w-10 h-10 text-primary" />
           </div>
        </div>

        <h1 className="text-4xl font-bold text-white mb-2 uppercase tracking-tighter">flspd</h1>
        <p className="text-on-surface-variant text-sm font-mono mb-10 tracking-widest uppercase opacity-70">Control Interface</p>

        {dashboardDisabled ? (
          <div className="space-y-6">
            <div className="p-4 rounded-xl bg-error/10 border border-error/20 flex items-start gap-3 text-left">
              <AlertTriangle className="w-5 h-5 text-error shrink-0 mt-0.5" />
              <p className="text-xs text-error leading-relaxed font-mono">
                Dashboard login is disabled on this backend: no admin Nostr pubkey is
                configured (<code>DASHBOARD_ADMIN_PUBKEY</code>). An operator needs to set it
                before anyone can authenticate.
              </p>
            </div>

            {info!.nodes.length > 0 && (
              <div className="p-4 rounded-xl bg-surface-container-low border border-outline-variant space-y-2 text-left">
                {info!.nodes.map((n) => (
                  <div key={n.name} className="flex items-center justify-between text-xs font-mono">
                    <span className="text-on-surface-variant">{n.name}</span>
                    <span className={n.connected ? 'text-secondary' : 'text-error'}>
                      {n.connected ? 'Connected' : 'Offline'}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>
        ) : (
          <form onSubmit={handleLogin} className="space-y-6">
            <div className="p-4 rounded-xl bg-surface-container-low border border-outline-variant flex items-start gap-3 text-left">
              <ShieldCheck className="w-5 h-5 text-secondary shrink-0 mt-0.5" />
              <p className="text-xs text-on-surface-variant leading-relaxed font-mono">
                Authentication required. Enter your Nostr private key (nsec) to sign NIP-98 events and access the dashboard.
              </p>
            </div>

            <div className="relative group">
              <div className="absolute inset-y-0 left-0 pl-4 flex items-center pointer-events-none text-on-surface-variant group-focus-within:text-primary transition-colors">
                <Lock className="w-4 h-4" />
              </div>
              <input
                type="password"
                placeholder="nsec1..."
                value={nsec}
                onChange={(e) => setNsec(e.target.value)}
                className="w-full bg-surface-container-lowest border border-outline rounded-xl py-4 pl-11 pr-4 text-sm text-white placeholder:text-on-surface-variant/30 focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/20 transition-all font-mono"
              />
            </div>

            {error && <p className="text-error text-[10px] uppercase tracking-wider font-bold animate-pulse">{error}</p>}

            <button
              type="submit"
              disabled={loading}
              className="w-full py-4 rounded-xl bg-primary text-background font-bold uppercase tracking-widest flex items-center justify-center gap-3 hover:bg-white disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-300 shadow-[0_0_30px_rgba(0,186,255,0.3)] hover:shadow-[0_0_50px_rgba(255,255,255,0.4)] active:scale-95 group"
            >
              <Key className="w-5 h-5 group-hover:rotate-12 transition-transform" />
              {loading ? 'Authenticating...' : 'Authorize Access'}
            </button>

            <p className="text-[10px] text-on-surface-variant/50 font-mono italic">
              Your key is only used locally to sign authentication events and is never sent to the server.
            </p>
          </form>
        )}
      </GlassCard>
    </div>
  );
};
