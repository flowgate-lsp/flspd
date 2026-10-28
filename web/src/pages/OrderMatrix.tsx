import React, { useState } from 'react';
import { GlassCard } from '../components/GlassCard';
import { Search, ExternalLink, RefreshCw, Loader2, AlertCircle, ChevronRight } from 'lucide-react';
import { useFormatAmount, cn } from '../lib/utils';
import { orderStatusPillClass, orderStatusDotClass } from '../lib/orderStatus';
import { useDashboard } from '../hooks/useDashboard';
import { useEvents } from '../hooks/useEvents';
import { useNavigate } from '@tanstack/react-router';

/* --- Utilities --- */

const middleTrim = (str: string, layout: number = 6) => {
  if (!str) return '...';
  if (str.length <= layout * 2) return str;
  return `${str.substring(0, layout)}...${str.substring(str.length - layout)}`;
};

const STAT_TILES: { key: string; label: string; tile: string }[] = [
  { key: 'PAID', label: 'In Progress', tile: 'stat-tile-primary' },
  { key: 'CREATED', label: 'Awaiting Payment', tile: 'stat-tile-tertiary' },
  { key: 'COMPLETED', label: 'Completed', tile: 'stat-tile-secondary' },
  { key: 'FAILED', label: 'Failed', tile: 'stat-tile-tertiary' },
  { key: 'EXPIRED', label: 'Expired', tile: 'stat-tile-neutral' },
];

interface OrderMatrixProps {
  onViewOrder: (id: string) => void;
  initialSearch?: string;
}

export const OrderMatrix: React.FC<OrderMatrixProps> = ({ onViewOrder, initialSearch = '' }) => {
  const { orders, loading, error, refreshOrders } = useDashboard();
  const { format } = useFormatAmount();
  const [search, setSearch] = useState(initialSearch);
  const [stateFilter, setStateFilter] = useState<string | null>(null);
  const navigate = useNavigate();

  React.useEffect(() => {
    setSearch(initialSearch);
  }, [initialSearch]);

  useEvents((event) => {
    if (event.type === 'order_state_changed') {
      refreshOrders();
    }
  });

  const stateCounts = orders.reduce((acc: Record<string, number>, o: any) => {
    acc[o.state] = (acc[o.state] || 0) + 1;
    return acc;
  }, {});

  const filteredOrders = orders.filter(order =>
    (!stateFilter || order.state === stateFilter) &&
    (order.order_id?.toLowerCase().includes(search.toLowerCase()) ||
    order.peer_id?.toLowerCase().includes(search.toLowerCase()) ||
    order.peer_alias?.toLowerCase().includes(search.toLowerCase()))
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
          className="mt-4 px-8 py-3 rounded-2xl bg-surface-container-high border border-outline-variant text-white text-[10px] font-black uppercase tracking-[0.2em] hover:bg-primary/20 hover:border-primary/20 transition-all duration-500 italic"
        >
          Force Re-Sync
        </button>
      </div>
    );
  }

  return (
    <div className="p-3 md:p-10 max-w-7xl mx-auto space-y-10 animate-in fade-in slide-in-from-bottom-2 duration-500">
      <header className="flex flex-col md:flex-row md:justify-between md:items-end gap-10">
        <div className="space-y-2">
          <h1 className="text-4xl md:text-5xl font-black tracking-tighter text-white uppercase italic leading-none">Order Matrix</h1>
          <p className="text-on-surface-variant font-mono text-[10px] tracking-[0.4em] uppercase font-black opacity-70">Inspect & Reconcile Liquidity Requests</p>
        </div>
        <div className="flex gap-4 items-center">
           <button 
             onClick={refreshOrders}
             className="px-5 py-3 rounded-2xl bg-surface-container-high border border-outline-variant text-[10px] font-black uppercase text-on-surface-variant hover:text-white hover:bg-primary/10 transition-all flex items-center gap-3 italic tracking-widest"
           >
             <RefreshCw className={cn("w-4 h-4", loading && "animate-spin")} /> Cache Sync
           </button>
        </div>
      </header>

      {/* Status Overview -- doubles as a filter */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
        {STAT_TILES.map((t) => (
          <button
            key={t.key}
            onClick={() => setStateFilter(stateFilter === t.key ? null : t.key)}
            className={cn("stat-tile", t.tile, stateFilter === t.key && "is-active")}
          >
            <div className="text-2xl font-black font-mono tabular-nums text-white leading-none">{stateCounts[t.key] || 0}</div>
            <div className="text-[9px] font-mono uppercase tracking-[0.2em] font-black text-on-surface-variant mt-2">{t.label}</div>
          </button>
        ))}
      </div>

      {/* Toolbar / Search */}
      <div className="flex flex-col md:flex-row items-stretch md:items-center gap-6">
        <GlassCard className="flex-1 p-0 border-outline-variant shadow-2xl bg-surface-container-lowest/60 overflow-hidden">
          <div className="px-6 py-4 flex items-center gap-4 group focus-within:bg-white/[0.03] transition-all">
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
        {stateFilter && (
          <button
            onClick={() => setStateFilter(null)}
            className="px-5 py-3 rounded-2xl bg-surface-container-high border border-outline-variant text-[10px] font-black uppercase text-on-surface-variant hover:text-white transition-all italic tracking-widest whitespace-nowrap"
          >
            Clear Filter
          </button>
        )}
      </div>

      {/* Orders Table - Desktop */}
      <div className="hidden lg:block">
        <GlassCard className="p-0 overflow-hidden border-outline-variant shadow-2xl bg-surface-variant/60">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="bg-surface-container-high/40">
                <th className="px-8 py-6 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.3em] font-black opacity-60">Protocol</th>
                <th className="px-8 py-6 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.3em] font-black opacity-60">Order ID</th>
                <th className="px-8 py-6 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.3em] font-black opacity-60">Node Integrity</th>
                <th className="px-8 py-6 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.3em] font-black opacity-60">Liquidity Depth</th>
                <th className="px-8 py-6 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.3em] font-black opacity-60">Status</th>
                <th className="px-8 py-6 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.3em] font-black opacity-60 text-right">Settlement</th>
              </tr>
            </thead>
            <tbody className="">
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
                    className="group hover:bg-white/[0.05] transition-all duration-500 cursor-pointer border-l-2 border-transparent hover:border-primary/40 active:bg-white/[0.08]"
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
                       <div className="flex items-center gap-3 group/peer">
                         {order.peer_color && (
                           <div 
                             className="w-1.5 h-1.5 rounded-full shrink-0 shadow-[0_0_8px_currentColor]" 
                             style={{ color: order.peer_color, backgroundColor: order.peer_color }} 
                           />
                         )}
                         <div className="flex flex-col">
                           <span className={cn(
                             "text-[11px] font-mono font-black uppercase italic tracking-wider transition-colors",
                             order.peer_alias ? "text-white" : "text-on-surface-variant/40"
                           )}>
                             {order.peer_alias || middleTrim(order.peer_id, 4)}
                           </span>
                           {order.peer_alias && (
                             <span className="text-[9px] font-mono text-on-surface-variant/40 truncate max-w-[100px]" title={order.peer_id}>
                               {middleTrim(order.peer_id, 6)}
                             </span>
                           )}
                         </div>
                       </div>
                    </td>
                    <td className="px-8 py-6">
                      <div className="flex flex-col gap-1">
                        <span className="text-base font-black text-white italic tracking-tighter leading-none">{format(order.lsp_balance_loki)}</span>
                        <span className="text-[9px] font-mono text-on-surface-variant/40 uppercase tracking-widest font-black italic">Fee: {format(order.amount, 'MLOKI')}</span>
                      </div>
                    </td>
                    <td className="px-8 py-6">
                      <span className={cn(
                        "text-[9px] font-black font-mono uppercase tracking-[0.2em] px-3.5 py-1.5 rounded-xl border italic shadow-sm transition-all duration-500",
                        orderStatusPillClass(order.state, order.payment_state)
                      )}>
                        {order.state}
                      </span>
                    </td>
                    <td className="px-8 py-6 text-right">
                       <div className="flex items-center justify-end gap-3 group/tx">
                          {order.funding_txid ? (
                            <a 
                              href={`https://lokichain.info/tx/${order.funding_txid}`}
                              target="_blank"
                              rel="noopener noreferrer"
                              title="View in Explorer" 
                              onClick={(e) => e.stopPropagation()}
                              className="text-[10px] font-mono text-primary/60 uppercase tracking-tighter hover:text-primary hover:underline underline-offset-4 transition-all italic"
                            >
                              {order.funding_txid.substring(0, 6)} • {order.funding_txid.slice(-6)}
                            </a>
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
      <div className="lg:hidden space-y-4">
        {filteredOrders.length === 0 ? (
          <div className="p-16 text-center border-2 border-dashed border-outline-variant rounded-3xl bg-surface-container-low/40">
            <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.4em] italic font-black opacity-30">No matching records found</p>
          </div>
        ) : (
          filteredOrders.map((order, i) => (
            <GlassCard 
              key={i} 
              onClick={() => onViewOrder(order.order_id)}
              className="p-0 border-outline-variant cursor-pointer active:scale-[0.97] transition-all duration-300 bg-surface-variant/70 flex flex-col group overflow-hidden"
            >
              {/* Header: Status Strip */}
              <div className="px-5 py-3.5 flex items-center justify-between bg-surface-container-high/40 border-b border-outline-variant">
                 <div className="flex items-center gap-3">
                    <span className={cn("w-1.5 h-1.5 rounded-full", orderStatusDotClass(order.state, order.payment_state))} />
                    <span className="text-[10px] font-mono font-black italic tracking-[0.2em] uppercase text-on-surface-variant group-hover:text-white transition-colors">{order.state}</span>
                 </div>
                 <span className="text-[9px] font-mono font-black italic text-primary border border-primary/20 bg-primary/5 px-2 py-0.5 rounded uppercase">LSPS1</span>
              </div>
            
              {/* Body: Order Details */}
              <div className="p-5 flex flex-col gap-6">
                {/* Identification */}
                <div className="flex flex-col gap-1">
                  <p className="text-[8px] font-mono text-on-surface-variant/40 uppercase tracking-[0.3em] font-black">Transmission ID</p>
                  <p className="text-xs font-mono font-bold tracking-tight text-white/90">{middleTrim(order.order_id, 10)}</p>
                </div>
                
                {/* Financial & Counterparty Grid */}
                <div className="grid grid-cols-2 gap-4 bg-surface-container-low/60 p-4 rounded-xl border border-outline-variant">
                  <div className="flex flex-col gap-1">
                    <span className="text-[8px] font-mono text-on-surface-variant/40 uppercase tracking-[0.3em] font-black">Capacity</span>
                    <span className="text-xl font-black text-white italic tracking-tighter tabular-nums leading-none">
                      {format(order.lsp_balance_loki).split(' ')[0]}
                      <span className="text-[9px] text-white/40 not-italic ml-1 uppercase font-mono tracking-widest">
                        {format(order.lsp_balance_loki).split(' ')[1]}
                      </span>
                    </span>
                    <span className="text-[8px] font-mono text-on-surface-variant/60 font-bold uppercase tracking-widest mt-1">Fee: {format(order.amount, 'MLOKI')}</span>
                  </div>
                  <div className="flex flex-col gap-1 overflow-hidden">
                    <span className="text-[8px] font-mono text-on-surface-variant/40 uppercase tracking-[0.3em] font-black">Counterparty</span>
                    <span className="text-[11px] font-black text-white italic tracking-tighter truncate leading-none mt-1">{order.peer_alias || 'Unknown Node'}</span>
                    <span className="text-[8px] font-mono text-on-surface-variant/60 font-bold italic tracking-widest truncate mt-1">{middleTrim(order.peer_id, 6)}</span>
                  </div>
                </div>
              </div>
            
              {/* Footer: Telemetry */}
              <div className="px-5 py-3 border-t border-outline-variant bg-white/[0.02] flex justify-between items-center mt-auto">
                 <p className="text-[9px] font-mono text-on-surface-variant/40 uppercase font-black italic tracking-widest">
                   {new Date(order.created_at).toLocaleString(undefined, { hour: 'numeric', minute: 'numeric', month: 'short', day: 'numeric' })}
                 </p>
                 <div className="flex items-center gap-4">
                   {order.funding_txid && (
                     <a 
                       href={`https://lokichain.info/tx/${order.funding_txid}`}
                       target="_blank"
                       rel="noopener noreferrer"
                       onClick={(e) => e.stopPropagation()}
                       className="text-[10px] font-mono text-primary/60 hover:text-primary transition-colors flex items-center gap-1.5 uppercase italic tracking-widest rounded px-2 py-1 bg-primary/5 border border-primary/10"
                     >
                        <ExternalLink className="w-3 h-3" /> TXID
                     </a>
                   )}
                   <ChevronRight className="w-4 h-4 text-white/20 group-hover:text-white/80 transition-colors" />
                 </div>
              </div>
            </GlassCard>
          ))
        )}
      </div>
    </div>
  );
};
