import React, { useState, useEffect } from 'react';
import { GlassCard } from '../components/GlassCard';
import { Users, Globe, ArrowUpRight, ArrowDownLeft, Search, ShieldCheck, Activity } from 'lucide-react';
import { useDashboard } from '../hooks/useDashboard';
import { formatSmart, cn } from '../lib/utils';

interface Peer {
  pub_key: string;
  address: string;
  inbound: boolean;
  active: boolean;
  in_sat: number;
  out_sat: number;
  order_count?: number;
}

interface PeerNodesProps {
  onViewPeerOrders?: (peerId: string) => void;
}

export const PeerNodes: React.FC<PeerNodesProps> = ({ onViewPeerOrders }) => {
  const { api } = useDashboard();
  const [peers, setPeers] = useState<Peer[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');

  useEffect(() => {
    const fetchPeers = async () => {
      try {
        const data = await api.listPeers();
        setPeers(data);
      } catch (error) {
        console.error('Failed to fetch peers:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchPeers();
  }, [api]);

  const filteredPeers = peers.filter(p => 
    (p.pub_key?.toLowerCase() || '').includes(search.toLowerCase()) || 
    (p.address?.toLowerCase() || '').includes(search.toLowerCase())
  );

  return (
    <div className="p-6 md:p-10 max-w-7xl mx-auto space-y-12 animate-in fade-in slide-in-from-bottom-2 duration-500">
      <header className="flex flex-col md:flex-row md:items-end justify-between gap-8 pb-8">
        <div className="space-y-2">
          <div className="flex items-center gap-4 mb-1">
             <div className="p-2.5 rounded-xl bg-primary/10 border border-primary/20 shadow-[0_0_15px_rgba(0,186,255,0.1)]">
               <Users className="w-6 h-6 text-primary" />
             </div>
             <h1 className="text-4xl md:text-5xl font-black text-white uppercase italic tracking-tighter leading-none">Peer Index</h1>
          </div>
          <p className="text-on-surface-variant font-mono text-[10px] uppercase tracking-[0.4em] font-black opacity-70">Lightning Network Counterparty Management</p>
        </div>
        
        <div className="flex gap-4">
          <GlassCard className="p-0 border-white/5 bg-surface-container-lowest/30 overflow-hidden">
            <div className="px-5 py-3 flex items-center gap-4 group focus-within:bg-white/[0.02] transition-all">
               <Search className="w-4 h-4 text-on-surface-variant group-focus-within:text-primary transition-colors" />
               <input 
                 type="text" 
                 placeholder="Search Pubkeys..." 
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
          {[1, 2, 3].map(i => (
            <div key={i} className="h-64 bg-white/[0.02] animate-pulse rounded-3xl border border-white/5 shadow-xl" />
          ))}
        </div>
      ) : filteredPeers.length === 0 ? (
        <div className="py-32 text-center rounded-3xl border-2 border-dashed border-white/5 bg-white/[0.01] flex flex-col items-center justify-center gap-6">
          <div className="p-6 rounded-full bg-white/5 text-on-surface-variant/20">
            <Globe className="w-16 h-16 opacity-20" />
          </div>
          <p className="text-on-surface-variant font-mono text-[10px] uppercase tracking-[0.4em] font-black italic opacity-40">No peer records identified in this sector</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
          {filteredPeers.map((peer, idx) => (
            <GlassCard
              key={peer.pub_key}
              className="p-8 border-white/5 hover:border-primary/30 transition-all duration-700 group relative overflow-hidden bg-surface-variant/20 hover:bg-white/[0.03]"
            >
              {/* Glow effect */}
              <div className={cn(
                "absolute -top-16 -right-16 w-32 h-32 blur-[60px] opacity-10 group-hover:opacity-30 transition-opacity duration-1000 rounded-full pointer-events-none",
                peer.active ? "bg-secondary" : "bg-error"
              )} />

              <div className="flex justify-between items-start mb-10">
                <div className="p-3.5 rounded-2xl bg-black/40 border border-white/5 group-hover:scale-110 transition-transform duration-500">
                  <Globe className={cn("w-5 h-5", peer.active ? "text-secondary" : "text-on-surface-variant/40")} />
                </div>
                <div className={cn(
                  "flex items-center gap-2.5 px-4 py-1.5 rounded-xl border italic shadow-sm transition-all duration-500",
                  peer.active ? "text-secondary border-secondary/20 bg-secondary/5" : "text-error border-error/20 bg-error/5"
                )}>
                  <Activity className={cn("w-1.5 h-1.5 rounded-full", peer.active ? "bg-secondary animate-pulse shadow-[0_0_8px_rgba(0,253,135,0.6)]" : "bg-error")} />
                  <span className="text-[10px] font-mono font-black uppercase tracking-[0.2em] leading-none">{peer.active ? 'Online' : 'Offline'}</span>
                </div>
              </div>

              <div className="mb-10 space-y-2">
                <h3 className="text-sm font-black text-white italic tracking-tighter flex items-center gap-2">
                  Peer Index #{idx + 1}
                  {peer.inbound && <ShieldCheck className="w-3.5 h-3.5 text-primary opacity-50" />}
                </h3>
                <p className="text-[10px] font-mono text-on-surface-variant/60 truncate group-hover:text-white transition-colors">
                  {peer.pub_key}
                </p>
              </div>

              <div className="grid grid-cols-2 gap-8 mb-10">
                <div className="space-y-2">
                  <p className="text-[9px] font-mono text-on-surface-variant/50 uppercase tracking-[0.3em] font-black italic">Sent</p>
                  <div className="flex items-center gap-2 text-white">
                    <ArrowUpRight className="w-3 h-3 text-secondary" />
                    <span className="text-[11px] font-black font-mono uppercase tracking-tighter">{formatSmart(peer.out_sat)}</span>
                  </div>
                </div>
                <div className="space-y-2">
                  <p className="text-[9px] font-mono text-on-surface-variant/50 uppercase tracking-[0.3em] font-black italic">Received</p>
                  <div className="flex items-center gap-2 text-white">
                    <ArrowDownLeft className="w-3 h-3 text-primary" />
                    <span className="text-[11px] font-black font-mono uppercase tracking-tighter">{formatSmart(peer.in_sat)}</span>
                  </div>
                </div>
              </div>

              <div className="pt-8 flex items-center justify-between">
                <div className="text-[9px] font-mono text-on-surface-variant/40 truncate max-w-[140px] italic font-black uppercase tracking-widest" title={peer.address}>
                  {peer.address || 'Direct Connect'}
                </div>
                {onViewPeerOrders && (
                  <button 
                    onClick={() => onViewPeerOrders(peer.pub_key)}
                    className="flex items-center gap-3 px-5 py-2.5 rounded-xl bg-white/5 border border-white/5 text-[10px] font-black uppercase italic tracking-widest text-on-surface-variant hover:text-white hover:bg-primary/10 transition-all active:scale-95 group/btn"
                  >
                    Orders
                    <span className="bg-primary/20 text-primary px-2 py-0.5 rounded-lg text-[9px] font-black group-hover/btn:bg-primary group-hover/btn:text-on-primary transition-all">
                      {peer.order_count || 0}
                    </span>
                  </button>
                )}
              </div>
            </GlassCard>
          ))}
        </div>
      )}
    </div>
  );
};
