import React, { useState } from 'react';
import { GlassCard } from '../components/GlassCard';
import { Shield, Zap, Search, Activity, MoreVertical, ExternalLink } from 'lucide-react';
import { useDashboard } from '../hooks/useDashboard';
import { formatSmart } from '../lib/utils';

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
    ch.peer_id?.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="p-4 md:p-10 max-w-7xl mx-auto space-y-8 animate-in fade-in duration-1000">
      <header className="flex flex-col md:flex-row md:items-center justify-between gap-6">
        <div>
          <h1 className="text-3xl font-bold text-white mb-2 uppercase italic tracking-tighter">JIT Registry</h1>
          <p className="text-on-surface-variant font-mono text-xs uppercase tracking-widest">Active Just-In-Time Incentivized Channels</p>
        </div>
        
        <div className="flex gap-4">
          <div className="glass-panel px-4 py-2 flex items-center gap-3">
             <Search className="w-4 h-4 text-on-surface-variant" />
             <input 
               type="text" 
               placeholder="Search registry..." 
               className="bg-transparent border-none outline-none text-xs text-white w-48 placeholder:text-on-surface-variant/40"
               value={search}
               onChange={(e) => setSearch(e.target.value)}
             />
          </div>
        </div>
      </header>

      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {[1, 2, 3].map(i => (
            <div key={i} className="h-64 bg-white/5 animate-pulse rounded-2xl border border-white/5" />
          ))}
        </div>
      ) : filteredChannels.length === 0 ? (
        <div className="p-20 text-center glass-panel border-dashed border-white/10 rounded-2xl">
          <Zap className="w-12 h-12 text-on-surface-variant/20 mx-auto mb-4" />
          <p className="text-on-surface-variant font-mono text-xs uppercase tracking-widest italic">No active JIT channels detected</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filteredChannels.map((ch, i) => (
            <GlassCard key={i} className="p-6 border-white/5 hover:border-primary/20 transition-all group relative overflow-hidden">
              {/* Status Glow */}
              <div className="absolute -top-12 -right-12 w-24 h-24 bg-primary/10 blur-3xl group-hover:bg-primary/20 transition-all rounded-full pointer-events-none" />
              
              <div className="flex justify-between items-start mb-6">
                <div className="p-3 rounded-xl bg-primary/10 border border-primary/20">
                  <Zap className="w-5 h-5 text-primary" />
                </div>
                <div className="flex items-center gap-1.5 px-3 py-1 rounded-full bg-secondary/10 border border-secondary/20">
                  <span className="w-1.5 h-1.5 rounded-full bg-secondary animate-glow-pulse" />
                  <span className="text-[10px] font-mono font-bold text-secondary uppercase tracking-widest leading-none">Active</span>
                </div>
              </div>

              <div className="space-y-1 mb-6">
                <h3 className="text-sm font-bold text-white uppercase tracking-wider flex items-center gap-2">
                  Channel Snapshot
                  <Shield className="w-3.5 h-3.5 text-on-surface-variant" />
                </h3>
                <p className="text-[10px] font-mono text-on-surface-variant truncate">{ch.channel_id || 'ID_PENDING_OPEN'}</p>
              </div>

              <div className="grid grid-cols-2 gap-4 mb-6">
                <div>
                  <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest mb-1 leading-none">Capacity</p>
                  <p className="text-lg font-black text-white italic tracking-tighter">
                    {formatSmart(ch.amount)}
                  </p>
                </div>
                <div>
                  <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest mb-1 leading-none">Asset Flow</p>
                  <div className="flex items-center gap-1.5 text-xs font-bold text-white">
                    <Activity className="w-3 h-3 text-secondary" />
                    Bilateral
                  </div>
                </div>
              </div>

              <div className="pt-6 border-t border-white/5 flex items-center justify-between">
                <div className="flex flex-col">
                  <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest mb-1 leading-none">Peer Index</p>
                  <p className="text-xs font-mono text-white/50 truncate max-w-[120px]">{ch.peer_id?.substring(0, 16)}...</p>
                </div>
                <div className="flex gap-2">
                   <button className="p-2 text-on-surface-variant hover:text-white hover:bg-white/5 rounded-lg border border-transparent hover:border-white/5 transition-all">
                     <ExternalLink className="w-4 h-4" />
                   </button>
                   <button className="p-2 text-on-surface-variant hover:text-white hover:bg-white/5 rounded-lg border border-transparent hover:border-white/5 transition-all">
                     <MoreVertical className="w-4 h-4" />
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
