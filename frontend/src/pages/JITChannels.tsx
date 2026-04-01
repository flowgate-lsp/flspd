import React, { useState } from 'react';
import { GlassCard } from '../components/GlassCard';
import { Shield, Zap, Search, Activity, ExternalLink, Globe, Lock, Cpu, ArrowUpRight } from 'lucide-react';
import { useDashboard } from '../hooks/useDashboard';
import { formatSmart, cn } from '../lib/utils';

export const JITChannels: React.FC = () => {
  const { api } = useDashboard();
  const [channels, setChannels] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');

  React.useEffect(() => {
    api.listJitChannels()
      .then(setChannels)
      .finally(() => setLoading(false));
  }, [api]);

  const filteredChannels = channels.filter(ch => 
    ch.channel_id?.toLowerCase().includes(search.toLowerCase()) ||
    ch.peer_id?.toLowerCase().includes(search.toLowerCase()) ||
    ch.order_id?.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="p-6 md:p-10 max-w-7xl mx-auto space-y-12 animate-in fade-in slide-in-from-bottom-8 duration-1000">
      <header className="flex flex-col md:flex-row md:items-end justify-between gap-8 pb-8 border-b border-white/5">
        <div className="space-y-2">
          <div className="flex items-center gap-4 mb-1">
             <div className="p-2.5 rounded-xl bg-secondary/10 border border-secondary/20 shadow-[0_0_15px_rgba(0,253,135,0.1)]">
               <Zap className="w-6 h-6 text-secondary" />
             </div>
             <h1 className="text-4xl md:text-5xl font-black text-white uppercase italic tracking-tighter leading-none">JIT Registry</h1>
          </div>
          <p className="text-on-surface-variant font-mono text-[10px] uppercase tracking-[0.4em] font-black opacity-70">Active Just-In-Time Incentivized Channels</p>
        </div>
        
        <div className="flex gap-4">
          <GlassCard className="p-0 border-white/5 bg-surface-container-lowest/30 overflow-hidden">
            <div className="px-5 py-3 flex items-center gap-4 group focus-within:bg-white/[0.02] transition-all">
               <Search className="w-4 h-4 text-on-surface-variant group-focus-within:text-secondary transition-colors" />
               <input 
                 type="text" 
                 placeholder="Filter Registry..." 
                 className="bg-transparent border-none outline-none text-[11px] font-mono text-white w-56 placeholder:text-on-surface-variant/30 uppercase tracking-widest"
                 value={search}
                 onChange={(e) => setSearch(e.target.value)}
               />
            </div>
          </GlassCard>
        </div>
      </header>

      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
          {[1, 2, 3, 4, 5, 6].map(i => (
            <div key={i} className="h-64 bg-white/[0.02] animate-pulse rounded-3xl border border-white/5 shadow-xl" />
          ))}
        </div>
      ) : filteredChannels.length === 0 ? (
        <div className="py-32 text-center rounded-3xl border-2 border-dashed border-white/5 bg-white/[0.01] flex flex-col items-center justify-center gap-6">
          <div className="p-6 rounded-full bg-white/5 text-on-surface-variant/20">
            <Globe className="w-16 h-16 opacity-20 animate-spin-slow" />
          </div>
          <p className="text-on-surface-variant font-mono text-[10px] uppercase tracking-[0.4em] font-black italic opacity-40">No active JIT telemetry detected in current sector</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
          {filteredChannels.map((ch, i) => (
            <GlassCard key={i} className="p-8 border-white/5 hover:border-secondary/30 transition-all duration-700 group relative overflow-hidden bg-surface-variant/20 hover:bg-white/[0.03]">
              {/* Animated Status Glow */}
              <div className={cn(
                "absolute -top-16 -right-16 w-32 h-32 blur-[60px] opacity-10 group-hover:opacity-30 transition-opacity duration-1000 rounded-full pointer-events-none",
                ch.status === 'ACTIVE' ? "bg-secondary" : "bg-error"
              )} />
              
              <div className="flex justify-between items-start mb-10">
                <div className="relative">
                  <div className="p-3.5 rounded-2xl bg-black/40 border border-white/5 group-hover:scale-110 transition-transform duration-500">
                    <Shield className="w-5 h-5 text-primary" />
                  </div>
                  <div className="absolute -bottom-1 -right-1 p-1 rounded-full bg-surface-container border border-white/10">
                    <Cpu className="w-2.5 h-2.5 text-on-surface-variant" />
                  </div>
                </div>
                
                <div className={cn(
                  "flex items-center gap-2.5 px-4 py-1.5 rounded-xl border italic shadow-sm transition-all duration-500",
                  ch.status === 'ACTIVE' ? "text-secondary border-secondary/20 bg-secondary/5" : "text-error border-error/20 bg-error/5"
                )}>
                  <span className={cn("w-1.5 h-1.5 rounded-full animate-glow-pulse shadow-[0_0_8px_currentColor]", ch.status === 'ACTIVE' ? "bg-secondary" : "bg-error")} />
                  <span className="text-[10px] font-mono font-black uppercase tracking-[0.2em] leading-none">{ch.status}</span>
                </div>
              </div>

              <div className="space-y-4 mb-10 relative z-10">
                <div className="space-y-1">
                   <p className="text-[9px] font-mono text-on-surface-variant/50 uppercase tracking-[0.3em] font-black">Registry ID</p>
                   <h3 className="text-xs font-black text-white uppercase tracking-wider flex items-center gap-2 font-mono truncate">
                     {ch.channel_id || 'OPEN_PENDING'}
                   </h3>
                </div>
                <div className="flex items-center gap-2 opacity-50 hover:opacity-100 transition-opacity cursor-help">
                   <Lock className="w-3 h-3 text-on-surface-variant" />
                   <p className="text-[9px] font-mono text-on-surface-variant uppercase tracking-widest leading-none font-bold">Encrypted Protocol</p>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-8 mb-10 relative z-10">
                <div className="space-y-2">
                  <p className="text-[9px] font-mono text-on-surface-variant/50 uppercase tracking-[0.2em] leading-none font-black">Capacity</p>
                  <p className="text-xl font-black text-white italic tracking-tighter leading-none group-hover:text-secondary transition-colors">
                    {formatSmart(ch.capacity)}
                  </p>
                </div>
                <div className="space-y-2">
                  <p className="text-[9px] font-mono text-on-surface-variant/50 uppercase tracking-[0.2em] leading-none font-black">Flow Logic</p>
                  <div className="flex items-center gap-2 text-[10px] font-black text-white uppercase italic tracking-widest">
                    <Activity className="w-3 h-3 text-secondary animate-pulse" />
                    Bilateral
                  </div>
                </div>
              </div>

              <div className="pt-8 border-t border-white/5 flex items-center justify-between relative z-10">
                <div className="flex flex-col gap-2">
                  <p className="text-[9px] font-mono text-on-surface-variant/40 uppercase tracking-[0.3em] leading-none font-black">Counterparty</p>
                  <p className="text-[10px] font-mono text-white/40 group-hover:text-white/70 transition-colors truncate max-w-[140px] italic font-bold">
                    {ch.peer_id?.substring(0, 16)}...
                  </p>
                </div>
                <div className="flex gap-3">
                   <a 
                     href={`https://lokichain.info/tx/${ch.txid}`}
                     target="_blank"
                     rel="noopener noreferrer"
                     className="p-3 text-on-surface-variant hover:text-white bg-white/5 hover:bg-secondary/20 rounded-2xl border border-white/5 transition-all shadow-xl active:scale-90"
                     title="View Transaction"
                   >
                     <ExternalLink className="w-4 h-4" />
                   </a>
                   <button className="p-3 text-on-surface-variant hover:text-white bg-white/5 hover:bg-primary/20 rounded-2xl border border-white/5 transition-all shadow-xl active:scale-90">
                     <ArrowUpRight className="w-4 h-4" />
                   </button>
                </div>
              </div>
            </GlassCard>
          ))}
        </div>
      )}
    </div>
  );
};
