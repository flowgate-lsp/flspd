import React, { useState, useEffect } from 'react';
import { GlassCard } from '../components/GlassCard';
import { Users, Globe, ArrowUpRight, ArrowDownLeft, Search, ShieldCheck, Activity, Copy, Check, Info } from 'lucide-react';
import { useDashboard } from '../hooks/useDashboard';
import { formatSmart, cn } from '../lib/utils';

/* --- Utilities --- */

interface Peer {
  pub_key: string;
  alias?: string;
  color?: string;
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
    (p.alias?.toLowerCase() || '').includes(search.toLowerCase()) ||
    (p.address?.toLowerCase() || '').includes(search.toLowerCase())
  );

  return (
    <div className="p-6 md:p-10 max-w-7xl mx-auto space-y-8 animate-in fade-in slide-in-from-bottom-2 duration-500">
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
}

const PeerCard: React.FC<PeerCardProps> = ({ peer, onViewOrders }) => {
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
      className="p-6 border-white/5 shadow-2xl bg-surface-variant/20 hover:border-white/10 group transition-all duration-500 overflow-hidden relative cursor-pointer active:scale-[0.99]"
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
        <div className="px-2.5 py-1 rounded-lg border border-white/5 bg-white/5 text-[9px] font-mono text-on-surface-variant uppercase tracking-widest font-black italic">
          {peer.active ? "Online" : "Offline"}
        </div>
      </div>

      {/* Identity Strip: Unified 'pubkey@address' Format */}
      <div 
        onClick={(e) => e.stopPropagation()}
        className="flex items-center gap-3 p-3 rounded-2xl bg-black/40 border border-white/5 mb-6 group-hover:bg-black/60 transition-all cursor-default"
      >
        <div className="flex items-center gap-3 flex-1 min-w-0">
          <ShieldCheck className="w-3.5 h-3.5 text-primary shrink-0 opacity-40" />
          <code className="text-[10px] font-mono text-white/50 break-all font-black tracking-widest uppercase">
             {peer.pub_key}@{peer.address ? peer.address.split(':')[0] : 'local'}
          </code>
        </div>
        <button 
          onClick={(e) => {
            e.stopPropagation();
            handleCopy();
          }}
          className="p-2 rounded-xl bg-white/5 text-on-surface-variant hover:text-white hover:bg-white/10 transition-all active:scale-90 group/copy shrink-0"
          title="Copy connect string (pubkey@address)"
        >
          {copied ? (
            <div className="flex items-center gap-2">
              <span className="text-[9px] font-black uppercase text-secondary">Copied!</span>
              <Check className="w-3.5 h-3.5 text-secondary" />
            </div>
          ) : (
            <Copy className="w-3.5 h-3.5 group-hover/copy:text-primary transition-colors" />
          )}
        </button>
      </div>

      {/* Clean Metrics Grid: Horizontal columns for high-density reading */}
      <div className="grid grid-cols-3 gap-4">
        <div className="space-y-1.5">
          <div className="flex items-center gap-1.5">
            <ArrowUpRight className="w-3 h-3 text-secondary" />
            <span className="text-[9px] font-mono text-on-surface-variant uppercase tracking-widest font-black opacity-40">Sent</span>
          </div>
          <div className="text-sm font-black text-white italic tracking-tight">{formatSmart(peer.out_sat)}</div>
        </div>

        <div className="space-y-1.5">
          <div className="flex items-center gap-1.5">
            <ArrowDownLeft className="w-3 h-3 text-primary" />
            <span className="text-[9px] font-mono text-on-surface-variant uppercase tracking-widest font-black opacity-40">Recv</span>
          </div>
          <div className="text-sm font-black text-white italic tracking-tight">{formatSmart(peer.in_sat)}</div>
        </div>

        <div className="space-y-1.5">
          <div className="flex items-center gap-1.5">
            <Activity className="w-3 h-3 text-tertiary" />
            <span className="text-[9px] font-mono text-on-surface-variant uppercase tracking-widest font-black opacity-40">Orders</span>
          </div>
          <div className="text-sm font-black text-white italic tracking-tight">{peer.order_count}</div>
        </div>
      </div>

      {/* Project Accent */}
      <div 
        className="absolute bottom-0 left-0 right-0 h-0.5 opacity-20" 
        style={{ backgroundColor: nodeColor }} 
      />
    </GlassCard>
  );
};
