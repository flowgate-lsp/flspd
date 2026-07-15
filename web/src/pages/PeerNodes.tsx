import React, { useState, useEffect } from 'react';
import { GlassCard } from '../components/GlassCard';
import { Users, Globe, ArrowUpRight, ArrowDownLeft, Search, ShieldCheck, Activity, Copy, Check } from 'lucide-react';
import { useDashboard } from '../hooks/useDashboard';
import { useFormatAmount, cn } from '../lib/utils';

/* --- Utilities --- */

interface Peer {
  pub_key: string;
  alias?: string;
  color?: string;
  address: string;
  inbound: boolean;
  active: boolean;
  in_loki: number;
  out_loki: number;
  order_count?: number;
}

interface PeerNodesProps {
  onViewPeerOrders?: (peerId: string) => void;
}

export const PeerNodes: React.FC<PeerNodesProps> = ({ onViewPeerOrders }) => {
  const { api } = useDashboard();
  const { format } = useFormatAmount();
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
    (p.alias?.toLowerCase() || '').includes(search.toLowerCase()) ||
    (p.address?.toLowerCase() || '').includes(search.toLowerCase())
  );

  return (
    <div className="p-3 md:p-10 max-w-7xl mx-auto space-y-8 animate-in fade-in slide-in-from-bottom-2 duration-500">
      <header className="flex flex-col md:flex-row md:items-end justify-between gap-4 pb-6">
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
                 placeholder="Search Pubkeys or Aliases..." 
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
          {[1, 2, 3].map(i => (
            <div key={i} className="h-64 bg-white/[0.02] animate-pulse rounded-3xl border border-white/5 shadow-xl" />
          ))}
        </div>
      ) : filteredPeers.length === 0 ? (
        <div className="py-20 text-center rounded-2xl border-2 border-dashed border-white/5 bg-white/[0.01] flex flex-col items-center justify-center gap-4">
          <div className="p-4 rounded-full bg-white/5 text-on-surface-variant/20">
            <Globe className="w-12 h-12 opacity-20" />
          </div>
          <p className="text-on-surface-variant font-mono text-[9px] uppercase tracking-[0.3em] font-black italic opacity-40">No peer records identified in this sector</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
          {filteredPeers.map((peer) => (
            <PeerCard
              key={peer.pub_key}
              peer={peer}
              onViewOrders={onViewPeerOrders}
              format={format}
            />
          ))}
        </div>
      )}
    </div>
  );
};

/* --- Sub-components & Helpers --- */

interface PeerCardProps {
  peer: Peer;
  onViewOrders?: (pubkey: string) => void;
  format: (value: number | string | undefined | null) => string;
}

const PeerCard: React.FC<PeerCardProps> = ({ peer, onViewOrders, format }) => {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    const connectString = `${peer.pub_key}@${peer.address || 'unknown'}`;
    navigator.clipboard.writeText(connectString);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const nodeColor = peer.color || '#00baff';

  return (
    <GlassCard 
      onClick={() => onViewOrders?.(peer.pub_key)}
      className="p-6 border-white/5 shadow-2xl bg-black/40 hover:bg-black/60 group transition-all duration-500 overflow-hidden relative cursor-pointer active:scale-[0.99] flex flex-col border-l-[4px]"
      style={{ borderLeftColor: nodeColor }}
    >
      {/* Header: Alias + Status Indicator */}
      <div className="flex items-center justify-between mb-5">
        <div className="flex items-center gap-3">
          <div className={cn(
             "w-2 h-2 rounded-full shadow-[0_0_8px_currentColor]",
             peer.active ? "bg-secondary text-secondary animate-pulse" : "bg-on-surface-variant/20 text-on-surface-variant"
          )} />
          <h3 className="text-lg font-bold text-white tracking-tight uppercase leading-none truncate max-w-[200px]">
            {peer.alias || "Unnamed Node"}
          </h3>
        </div>
        <div className="px-2.5 py-1 rounded-sm border border-white/5 bg-white/5 text-[9px] font-mono text-on-surface-variant uppercase tracking-widest font-black">
          {peer.active ? "Online" : "Offline"}
        </div>
      </div>

      {/* The Hash Plate (Identity block) */}
      <div 
        onClick={(e) => e.stopPropagation()}
        className="mb-6 p-4 rounded-lg bg-black/60 border border-white/10 flex flex-col gap-3 relative cursor-text text-left"
      >
        <div className="flex items-center justify-between w-full">
           <div className="flex items-center gap-2 text-white/30">
               <ShieldCheck className="w-3.5 h-3.5" />
               <span className="text-[9px] font-mono uppercase tracking-[0.2em] font-black">Routing ID</span>
           </div>
           <button 
             onClick={(e) => {
               e.stopPropagation();
               handleCopy();
             }}
             className="text-white/30 hover:text-white transition-colors active:scale-90"
             title="Copy connect string"
           >
              {copied ? <Check className="w-3.5 h-3.5 text-secondary" /> : <Copy className="w-3.5 h-3.5" />}
           </button>
        </div>
        <div className="font-mono text-[11px] leading-[1.6] select-text pb-1">
           <span className="text-white/80 break-all">{peer.pub_key}</span>
           <div className="mt-2 flex items-center gap-2 bg-white/5 w-fit px-2 py-0.5 rounded opacity-80">
             <span className="text-white/30">@</span>
             <span className="text-secondary/90 tracking-wider font-semibold">{peer.address ? peer.address.split(':')[0] : 'local'}</span>
           </div>
        </div>
      </div>

      {/* Metrics Array */}
      <div className="grid grid-cols-3 gap-2 mt-auto">
        <div className="space-y-1.5">
          <span className="text-[9px] font-mono text-white/30 uppercase tracking-widest font-black flex items-center gap-1.5"><ArrowUpRight className="w-3 h-3 text-secondary"/> Sent</span>
          <div className="text-sm font-medium text-white tracking-tight tabular-nums">{format(peer.out_loki)}</div>
        </div>
        <div className="space-y-1.5 border-l border-white/5 pl-3">
          <span className="text-[9px] font-mono text-white/30 uppercase tracking-widest font-black flex items-center gap-1.5"><ArrowDownLeft className="w-3 h-3 text-primary"/> Recv</span>
          <div className="text-sm font-medium text-white tracking-tight tabular-nums">{format(peer.in_loki)}</div>
        </div>
        <div className="space-y-1.5 border-l border-white/5 pl-3">
          <span className="text-[9px] font-mono text-white/30 uppercase tracking-widest font-black flex items-center gap-1.5"><Activity className="w-3 h-3 text-tertiary"/> Orders</span>
          <div className="text-sm font-medium text-white tracking-tight tabular-nums">{peer.order_count}</div>
        </div>
      </div>
    </GlassCard>
  );
};
