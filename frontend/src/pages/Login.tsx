import React from 'react';
import { GlassCard } from '../components/GlassCard';
import { Activity, ShieldCheck, Key } from 'lucide-react';
import { useAuth } from '../hooks/useAuth';

export const Login: React.FC = () => {
  const { login } = useAuth();

  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-6 transition-all duration-700">
      {/* Background kinetic effect */}
      <div className="fixed inset-0 overflow-hidden pointer-events-none">
        <div className="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary/10 blur-[120px] rounded-full animate-pulse" />
        <div className="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-secondary/10 blur-[120px] rounded-full animate-pulse delay-700" />
      </div>

      <GlassCard className="w-full max-w-md p-10 text-center relative z-10 border border-white/10 hover:border-primary/30 transition-colors duration-500">
        <div className="flex justify-center mb-8">
           <div className="w-20 h-20 rounded-2xl bg-primary/10 flex items-center justify-center animate-glow-pulse border border-primary/20 shadow-[0_0_40px_rgba(0,186,255,0.2)]">
             <Activity className="w-10 h-10 text-primary" />
           </div>
        </div>

        <h1 className="text-4xl font-bold text-white mb-2 uppercase tracking-tighter">flspd</h1>
        <p className="text-on-surface-variant text-sm font-mono mb-10 tracking-widest uppercase opacity-70">Control Interface</p>

        <div className="space-y-6">
          <div className="p-4 rounded-xl bg-surface-container-low border border-white/5 flex items-start gap-3 text-left">
            <ShieldCheck className="w-5 h-5 text-secondary shrink-0 mt-0.5" />
            <p className="text-xs text-on-surface-variant leading-relaxed font-mono">
              Authentication required. Sign a NIP-98 event with your Nostr key to access the node dashboard.
            </p>
          </div>
          
          <button 
            onClick={login}
            className="w-full py-4 rounded-xl bg-primary text-background font-bold uppercase tracking-widest flex items-center justify-center gap-3 hover:bg-white transition-all duration-300 shadow-[0_0_30px_rgba(0,186,255,0.3)] hover:shadow-[0_0_50px_rgba(255,255,255,0.4)] active:scale-95 group"
          >
            <Key className="w-5 h-5 group-hover:rotate-12 transition-transform" />
            Connect via Nostr
          </button>
          
          <p className="text-[10px] text-on-surface-variant/50 font-mono italic">
            Compatible with NIP-07 browser extensions like Alby, nos2x, or ZEBEDEE.
          </p>
        </div>
      </GlassCard>
    </div>
  );
};
