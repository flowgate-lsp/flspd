import React, { useState } from 'react';
import { GlassCard } from '../components/GlassCard';
import { Search, ExternalLink, RefreshCw, Loader2, AlertCircle, ListFilter, ChevronRight, Download } from 'lucide-react';
import { cn, formatSmart, formatLoki } from '../lib/utils';
import { useDashboard } from '../hooks/useDashboard';
import { useEvents } from '../hooks/useEvents';
import { useNavigate } from '@tanstack/react-router';

interface OrderMatrixProps {
  onViewOrder: (id: string) => void;
  initialSearch?: string;
}

export const OrderMatrix: React.FC<OrderMatrixProps> = ({ onViewOrder, initialSearch = '' }) => {
  const { orders, loading, error, refreshOrders } = useDashboard();
  const [search, setSearch] = useState(initialSearch);
  const navigate = useNavigate();

  React.useEffect(() => {
    setSearch(initialSearch);
  }, [initialSearch]);

  useEvents((event) => {
    if (event.type === 'order_state_changed') {
      refreshOrders();
    }
  });

  const filteredOrders = orders.filter(order => 
    order.order_id?.toLowerCase().includes(search.toLowerCase()) ||
    order.peer_id?.toLowerCase().includes(search.toLowerCase())
  );

  if (loading && orders.length === 0) {
    return (
      <div className="h-[60vh] flex items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <Loader2 className="w-10 h-10 text-primary animate-spin" />
          <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.4em] font-black animate-pulse">Scanning Matrix...</p>
        </div>
      </div>
    );
  }

  if (error && orders.length === 0) {
    return (
      <div className="h-[60vh] flex flex-col items-center justify-center gap-6 text-center px-10">
        <div className="p-4 rounded-full bg-error/5 border border-error/20">
          <AlertCircle className="w-12 h-12 text-error animate-pulse" />
        </div>
        <div className="space-y-2">
          <h2 className="text-2xl font-black text-white uppercase italic tracking-tighter">Sync Interrupted</h2>
          <p className="text-on-surface-variant font-mono text-[10px] uppercase tracking-widest max-w-sm opacity-60 leading-relaxed">{error}</p>
        </div>
        <button 
          onClick={refreshOrders}
          className="mt-4 px-8 py-3 rounded-2xl bg-white/5 border border-white/5 text-white text-[10px] font-black uppercase tracking-[0.2em] hover:bg-primary/20 hover:border-primary/20 transition-all duration-500 italic"
        >
          Force Re-Sync
        </button>
      </div>
    );
  }

  return (
    <div className="p-6 md:p-10 max-w-7xl mx-auto space-y-10 animate-in fade-in slide-in-from-bottom-6 duration-1000">
      <header className="flex flex-col md:flex-row md:justify-between md:items-end gap-10">
        <div className="space-y-2">
          <h1 className="text-4xl md:text-5xl font-black tracking-tighter text-white uppercase italic leading-none">Order Matrix</h1>
          <p className="text-on-surface-variant font-mono text-[10px] tracking-[0.4em] uppercase font-black opacity-70">Inspect & Reconcile Liquidity Requests</p>
        </div>
        <div className="flex gap-4 items-center">
           <button 
             onClick={refreshOrders}
             className="px-5 py-3 rounded-2xl bg-white/5 border border-white/5 text-[10px] font-black uppercase text-on-surface-variant hover:text-white hover:bg-primary/10 transition-all flex items-center gap-3 italic tracking-widest"
           >
             <RefreshCw className={cn("w-4 h-4", loading && "animate-spin")} /> Cache Sync
           </button>
           <button className="px-6 py-3 rounded-2xl bg-primary text-on-primary font-black italic uppercase text-[10px] tracking-[0.2em] flex items-center gap-3 hover:shadow-[0_0_20px_rgba(0,186,255,0.3)] transition-all active:scale-95">
             <Download className="w-4 h-4" /> Export Store
           </button>
        </div>
      </header>

      {/* Toolbar / Search */}
      <div className="flex flex-col md:flex-row items-stretch md:items-center gap-6">
        <GlassCard className="flex-1 p-0 border-white/5 shadow-2xl bg-surface-container-lowest/30 overflow-hidden">
          <div className="px-6 py-4 flex items-center gap-4 group focus-within:bg-white/[0.02] transition-all">
            <Search className="w-5 h-5 text-on-surface-variant group-focus-within:text-primary transition-colors" />
            <input 
              type="text" 
              value={search}
              onChange={(e) => {
                const val = e.target.value;
                setSearch(val);
                navigate({ to: '/orders', search: { q: val }, replace: true });
              }}
              placeholder="Search by Order ID or Peer Pubkey..." 
              className="bg-transparent border-none outline-none text-sm text-white w-full placeholder:text-on-surface-variant/30 font-mono tracking-tight"
            />
          </div>
        </GlassCard>
        <button className="px-8 py-4 rounded-2xl bg-white/5 border border-white/10 flex items-center justify-center gap-3 text-[10px] font-black italic text-white uppercase tracking-[0.2em] hover:bg-white/10 transition-all duration-300">
          <ListFilter className="w-4 h-4 text-primary" /> Filter Matrix
        </button>
      </div>

      {/* Orders Table - Desktop */}
      <div className="hidden lg:block">
        <GlassCard className="p-0 overflow-hidden border-white/5 shadow-2xl bg-surface-variant/20">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-white/5 bg-white/[0.02]">
                <th className="px-8 py-6 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.3em] font-black opacity-60">Protocol</th>
                <th className="px-8 py-6 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.3em] font-black opacity-60">Order ID</th>
                <th className="px-8 py-6 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.3em] font-black opacity-60">Node Integrity</th>
                <th className="px-8 py-6 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.3em] font-black opacity-60">Liquidity Depth</th>
                <th className="px-8 py-6 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.3em] font-black opacity-60">Status</th>
                <th className="px-8 py-6 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.3em] font-black opacity-60 text-right">Settlement</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-white/5">
              {filteredOrders.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-8 py-32 text-center text-on-surface-variant font-mono text-[10px] uppercase tracking-[0.4em] font-black opacity-30 italic">
                    No transmission records identified in this sector
                  </td>
                </tr>
              ) : (
                filteredOrders.map((order, i) => (
                  <tr 
                    key={i} 
                    onClick={() => onViewOrder(order.order_id)}
                    className="group hover:bg-white/[0.04] transition-all duration-500 cursor-pointer border-l-2 border-transparent hover:border-primary/40 active:bg-white/[0.08]"
                  >
                    <td className="px-8 py-6">
                      <span className="text-[9px] font-mono font-black italic text-primary border border-primary/30 bg-primary/10 px-2.5 py-1 rounded-lg tracking-widest">
                        LSPS1
                      </span>
                    </td>
                    <td className="px-8 py-6">
                      <span className="text-xs font-mono font-bold text-white/80 group-hover:text-white transition-colors tracking-tight">
                        {order.order_id}
                      </span>
                    </td>
                    <td className="px-8 py-6">
                       <span className="text-[11px] font-mono text-on-surface-variant/70 block whitespace-nowrap opacity-60 group-hover:opacity-100 transition-opacity" title={order.peer_id}>
                         {order.peer_id}
                       </span>
                    </td>
                    <td className="px-8 py-6">
                      <div className="flex flex-col gap-1">
                        <span className="text-base font-black text-white italic tracking-tighter leading-none">{formatSmart(order.lsp_balance_loki)}</span>
                        <span className="text-[9px] font-mono text-on-surface-variant/40 uppercase tracking-widest font-black italic">Fee: {formatLoki(order.amount)}</span>
                      </div>
                    </td>
                    <td className="px-8 py-6">
                      <span className={cn(
                        "text-[9px] font-black font-mono uppercase tracking-[0.2em] px-3.5 py-1.5 rounded-xl border italic shadow-sm transition-all duration-500",
                        order.state === 'COMPLETED' ? "text-secondary border-secondary/30 bg-secondary/5 group-hover:bg-secondary/10" :
                        order.state === 'FAILED' ? "text-error border-error/30 bg-error/5 group-hover:bg-error/10" :
                        order.state === 'PAID' ? "text-primary border-primary/30 bg-primary/5 group-hover:bg-primary/10" :
                        "text-tertiary border-tertiary/30 bg-tertiary/5 group-hover:bg-tertiary/10"
                      )}>
                        {order.state}
                      </span>
                    </td>
                    <td className="px-8 py-6 text-right">
                       <div className="flex items-center justify-end gap-3 group/tx">
                          {order.funding_txid ? (
                            <>
                              <span className="text-[10px] font-mono text-on-surface-variant/60 uppercase tracking-tighter group-hover:text-white/40 transition-colors">
                                {order.funding_txid.substring(0, 6)} • {order.funding_txid.slice(-6)}
                              </span>
                              <a 
                                href={`https://lokichain.info/tx/${order.funding_txid}`}
                                target="_blank"
                                rel="noopener noreferrer"
                                title="View in Explorer" 
                                onClick={(e) => e.stopPropagation()}
                                className="p-2.5 text-on-surface-variant hover:text-white hover:bg-primary/20 hover:border-primary/20 rounded-xl transition-all border border-white/5 bg-white/5 flex items-center justify-center active:scale-90"
                              >
                                 <ExternalLink className="w-4 h-4" />
                              </a>
                            </>
                          ) : (
                            <span className="text-[10px] font-mono text-on-surface-variant/20 uppercase italic tracking-widest">Pending Sync</span>
                          )}
                       </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </GlassCard>
      </div>

      {/* Orders List - Mobile */}
      <div className="lg:hidden space-y-6">
        {filteredOrders.length === 0 ? (
          <div className="p-16 text-center border-2 border-dashed border-white/5 rounded-3xl bg-white/[0.02]">
            <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.4em] italic font-black opacity-30">No matching records found</p>
          </div>
        ) : (
          filteredOrders.map((order, i) => (
            <GlassCard 
              key={i} 
              onClick={() => onViewOrder(order.order_id)}
              className="p-7 border-white/5 cursor-pointer active:scale-[0.97] transition-all duration-300 bg-surface-variant/30"
            >
              <div className="flex justify-between items-start mb-6">
                <div className="flex items-center gap-3">
                  <span className="text-[9px] font-mono font-black italic text-primary border border-primary/20 bg-primary/5 px-2 py-0.5 rounded-lg uppercase">LSPS1</span>
                  <span className="text-[11px] font-mono text-white font-bold tracking-tight">{order.order_id}</span>
                </div>
                <span className={cn(
                  "text-[9px] font-black font-mono uppercase tracking-[0.2em] px-2.5 py-1.5 rounded-xl border italic",
                  order.state === 'COMPLETED' ? "text-secondary border-secondary/20 bg-secondary/5" :
                  order.state === 'FAILED' ? "text-error border-error/20 bg-error/5" :
                  "text-primary border-primary/20 bg-primary/5"
                )}>
                  {order.state}
                </span>
              </div>
              
              <div className="grid grid-cols-2 gap-8 mb-6">
                <div className="space-y-2">
                  <p className="text-[9px] font-mono text-on-surface-variant/50 uppercase tracking-[0.2em] leading-none font-black">Capacity Inbound</p>
                  <p className="text-xl font-black text-white italic tracking-tighter leading-none">{formatSmart(order.lsp_balance_loki)}</p>
                  <p className="text-[10px] font-mono text-on-surface-variant opacity-60 font-black italic tracking-widest">Fee: {formatLoki(order.amount)}</p>
                </div>
                <div className="space-y-2 overflow-hidden">
                  <p className="text-[9px] font-mono text-on-surface-variant/50 uppercase tracking-[0.2em] leading-none font-black">Counterparty</p>
                  <p className="text-[10px] font-mono text-white break-all tracking-tighter opacity-70 italic">{order.peer_id.substring(0, 32)}...</p>
                </div>
              </div>
              
              <div className="pt-5 border-t border-white/5 flex justify-between items-center">
                 <p className="text-[9px] font-mono text-on-surface-variant/40 uppercase font-black italic tracking-widest">
                   {new Date(order.created_at).toLocaleString()}
                 </p>
                 <div className="flex items-center gap-3">
                   {order.funding_txid && (
                     <a 
                       href={`https://lokichain.info/tx/${order.funding_txid}`}
                       target="_blank"
                       rel="noopener noreferrer"
                       onClick={(e) => e.stopPropagation()}
                       className="p-3 text-on-surface-variant bg-white/5 rounded-2xl border border-white/5 hover:text-white transition-all active:scale-90"
                     >
                       <ExternalLink className="w-4 h-4" />
                     </a>
                   )}
                   <ChevronRight className="w-4 h-4 text-on-surface-variant/20" />
                 </div>
              </div>
            </GlassCard>
          ))
        )}
      </div>
    </div>
  );
};
