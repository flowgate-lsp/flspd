import React, { useState } from 'react';
import { GlassCard } from '../components/GlassCard';
import { Shield, Zap, Search, Activity, ExternalLink, Globe, Lock, Cpu, Copy } from 'lucide-react';
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
    <div className="p-6 md:p-10 max-w-7xl mx-auto space-y-8 animate-in fade-in slide-in-from-bottom-2 duration-500">
      <header className="flex flex-col md:flex-row md:items-end justify-between gap-4 pb-6">
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
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {[1, 2, 3, 4, 5, 6].map(i => (
            <div key={i} className="h-64 bg-white/[0.02] animate-pulse rounded-3xl border border-white/5 shadow-xl" />
          ))}
        </div>
      ) : filteredChannels.length === 0 ? (
        <div className="py-20 text-center rounded-2xl border-2 border-dashed border-white/5 bg-white/[0.01] flex flex-col items-center justify-center gap-4">
          <div className="p-4 rounded-full bg-white/5 text-on-surface-variant/20">
            <Globe className="w-12 h-12 opacity-20 animate-spin-slow" />
          </div>
          <p className="text-on-surface-variant font-mono text-[9px] uppercase tracking-[0.3em] font-black italic opacity-40">No active JIT telemetry detected in current sector</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filteredChannels.map((ch, i) => (
            <GlassCard 
              key={i} 
              className={cn(
                "p-6 border-white/5 transition-all duration-500 group relative overflow-hidden bg-black/40 hover:bg-black/60 flex flex-col border-l-[4px]",
                ch.status === 'ACTIVE' ? "border-l-secondary" : "border-l-error"
              )}
            >
              <div className="flex justify-between items-start mb-6">
                <div className="relative">
                  <div className="p-3 rounded-2xl bg-black/40 border border-white/5 group-hover:scale-105 transition-transform duration-500 text-primary">
                    <Shield className="w-5 h-5" />
                  </div>
                  <div className="absolute -bottom-1 -right-1 p-1 rounded-full bg-surface-container border border-white/10">
                    <Cpu className="w-2 h-2 text-on-surface-variant" />
                  </div>
                </div>
                
                <div className={cn(
                  "flex items-center gap-2 px-3.5 py-1.5 rounded-sm border shadow-sm transition-all duration-500",
                  ch.status === 'ACTIVE' ? "text-secondary border-secondary/20 bg-secondary/5" : "text-error border-error/20 bg-error/5"
                )}>
                  <span className={cn("w-1.5 h-1.5 rounded-full shadow-[0_0_8px_currentColor]", ch.status === 'ACTIVE' ? "bg-secondary animate-pulse" : "bg-error")} />
                  <span className="text-[9px] font-mono font-black uppercase tracking-[0.2em] leading-none">{ch.status}</span>
                </div>
              </div>

              {/* Counterparty Hash Plate */}
              <div className="mb-6 p-4 rounded-lg bg-black/60 border border-white/10 flex flex-col gap-3 relative cursor-text text-left">
                <div className="flex items-center justify-between w-full">
                   <div className="flex items-center gap-2 text-white/30">
                       <Shield className="w-3.5 h-3.5" />
                       <span className="text-[9px] font-mono uppercase tracking-[0.2em] font-black">Counterparty ID</span>
                   </div>
                   {ch.peer_id && (
                     <button 
                       onClick={(e) => {
                         e.stopPropagation();
                         navigator.clipboard.writeText(ch.peer_id);
                         /* In production, integrate local `copied` state per row if desired. I'll rely on native copy feedback for now */
                       }}
                       className="text-white/30 hover:text-white transition-colors active:scale-90"
                       title="Copy Peer ID"
                     >
                        <Copy className="w-3.5 h-3.5" />
                     </button>
                   )}
                </div>
                <div className="font-mono text-[11px] leading-[1.6] select-text pb-1 text-white/80 break-all">
                  {ch.peer_id ? ch.peer_id : <span className="text-white/20 italic">AWAITING PEER SYNC</span>}
                </div>
              </div>

              <div className="grid grid-cols-2 gap-8 mb-6 relative z-10 w-full">
                <div className="space-y-1.5">
                  <p className="text-[8px] font-mono text-on-surface-variant/50 uppercase tracking-[0.2em] font-black italic">Capacity</p>
                  <p className="text-lg font-black text-white tracking-tighter leading-none group-hover:text-secondary transition-colors tabular-nums">
                    {formatSmart(ch.capacity)}
                  </p>
                </div>
                <div className="space-y-1.5 border-l border-white/5 pl-4">
                  <p className="text-[8px] font-mono text-on-surface-variant/50 uppercase tracking-[0.2em] font-black italic">Flow Logic</p>
                  <div className="flex items-center gap-2 text-[9px] font-black text-white uppercase italic tracking-widest leading-none mt-1">
                    <Activity className="w-3 h-3 text-secondary animate-pulse" />
                    Bilateral
                  </div>
                </div>
              </div>

              <div className="pt-5 flex items-center justify-between border-t border-white/5 relative z-10 mt-auto">
                <div className="flex flex-col gap-1.5">
                  <p className="text-[8px] font-mono text-on-surface-variant/40 uppercase tracking-[0.2em] font-black italic flex items-center gap-1.5"><Lock className="w-2 h-2"/> Registry Source</p>
                  <p className="text-[9px] font-mono text-white/40 group-hover:text-white/70 transition-colors truncate max-w-[130px] font-bold">
                    {ch.channel_id || 'OPEN_PENDING'}
                  </p>
                </div>
                 <div className="flex gap-3">
                    <a 
                      href={`https://lokichain.info/tx/${ch.txid}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="p-2 text-on-surface-variant hover:text-white bg-white/5 hover:bg-secondary/20 rounded-lg border border-white/5 transition-all shadow-xl active:scale-95"
                      title="View Transaction"
                    >
                      <ExternalLink className="w-4 h-4" />
                    </a>
                 </div>
              </div>
            </GlassCard>
          ))}
        </div>
      )}
    </div>
  );
};
