import React, { useState } from 'react';
import { GlassCard } from '../components/GlassCard';
import { Search, Filter, Download, MoreHorizontal, ExternalLink, RefreshCw, Loader2, AlertCircle } from 'lucide-react';
import { cn, formatSats } from '../lib/utils';
import { useDashboard } from '../hooks/useDashboard';
import { useEvents } from '../hooks/useEvents';

interface OrderMatrixProps {
  onViewOrder: (id: string) => void;
}

export const OrderMatrix: React.FC<OrderMatrixProps> = ({ onViewOrder }) => {
  const { orders, loading, error, refreshOrders } = useDashboard();
  const [search, setSearch] = useState('');

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
        <Loader2 className="w-8 h-8 text-primary animate-spin" />
      </div>
    );
  }

  if (error && orders.length === 0) {
    return (
      <div className="h-[60vh] flex flex-col items-center justify-center gap-4 text-center px-6">
        <AlertCircle className="w-12 h-12 text-error animate-bounce" />
        <h2 className="text-xl font-bold text-white uppercase italic tracking-widest">Database Sync Failed</h2>
        <p className="text-on-surface-variant font-mono text-xs max-w-sm">{error}</p>
        <button 
          onClick={refreshOrders}
          className="mt-4 px-6 py-2 rounded-xl bg-primary/10 border border-primary/20 text-primary text-xs font-mono font-bold uppercase tracking-widest hover:bg-primary/20 transition-all"
        >
          Retry Connection
        </button>
      </div>
    );
  }

  return (
    <div className="p-4 md:p-10 max-w-7xl mx-auto space-y-8 animate-in fade-in duration-1000">
      <header className="flex flex-col md:flex-row md:justify-between md:items-end gap-6">
        <div>
          <h1 className="text-3xl md:text-4xl font-bold tracking-tighter text-white mb-2 uppercase italic leading-none">Order Matrix</h1>
          <p className="text-on-surface-variant font-mono text-xs md:text-sm tracking-widest uppercase">Inspect & Reconcile Liquidity Requests</p>
        </div>
        <div className="flex gap-3 overflow-x-auto pb-2 md:pb-0">
           <button 
             onClick={refreshOrders}
             className="glass-panel px-4 py-2.5 text-[10px] font-mono font-bold uppercase text-on-surface-variant hover:text-white flex items-center gap-2 transition-all whitespace-nowrap"
           >
             <RefreshCw className={cn("w-3.5 h-3.5", loading && "animate-spin")} /> Force Sync
           </button>
           <button className="btn-primary flex items-center gap-2 text-[10px] uppercase font-mono tracking-widest px-6 py-2.5 whitespace-nowrap">
             <Download className="w-4 h-4" /> Export CSV
           </button>
        </div>
      </header>

      {/* Toolbar */}
      <div className="flex flex-col md:flex-row items-stretch md:items-center gap-4">
        <div className="flex-1 glass-panel px-4 py-3 flex items-center gap-3 group focus-within:border-primary/40 transition-colors">
          <Search className="w-4 h-4 text-on-surface-variant group-focus-within:text-primary transition-colors" />
          <input 
            type="text" 
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by Order ID or Peer Pubkey..." 
            className="bg-transparent border-none outline-none text-sm text-white w-full placeholder:text-on-surface-variant/40"
          />
        </div>
        <button className="glass-panel px-6 py-3 flex items-center justify-center gap-2 text-xs font-mono font-bold text-white hover:bg-white/5 transition-all uppercase tracking-widest">
          <Filter className="w-4 h-4" /> Filter Status
        </button>
      </div>

      {/* Orders Table - Desktop */}
      <div className="hidden lg:block">
        <GlassCard className="p-0 overflow-hidden border-white/5">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-white/5 bg-white/5">
                <th className="px-6 py-5 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.2em] font-bold">Protocol</th>
                <th className="px-6 py-5 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.2em] font-bold">Order ID</th>
                <th className="px-6 py-5 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.2em] font-bold">Node ID</th>
                <th className="px-6 py-5 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.2em] font-bold">Liquidity</th>
                <th className="px-6 py-5 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.2em] font-bold">State</th>
                <th className="px-6 py-5 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.2em] font-bold text-right">Reference</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-white/5">
              {filteredOrders.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-6 py-20 text-center text-on-surface-variant font-mono text-xs uppercase tracking-widest opacity-50">
                    No matching transmission records
                  </td>
                </tr>
              ) : (
                filteredOrders.map((order, i) => (
                  <tr 
                    key={i} 
                    onClick={() => onViewOrder(order.order_id)}
                    className="group hover:bg-white/[0.03] transition-colors cursor-pointer"
                  >
                    <td className="px-6 py-4">
                      <span className="text-[10px] font-mono font-bold text-primary border border-primary/20 bg-primary/5 px-2 py-0.5 rounded italic">
                        LSPS1
                      </span>
                    </td>
                    <td className="px-6 py-4">
                      <span className="text-xs font-mono text-white/90 group-hover:text-white transition-colors">
                        {order.order_id}
                      </span>
                    </td>
                    <td className="px-6 py-4">
                       <span className="text-xs font-mono text-on-surface-variant truncate block max-w-[120px]" title={order.peer_id}>
                         {order.peer_id?.substring(0, 8)}...{order.peer_id?.slice(-8)}
                       </span>
                    </td>
                    <td className="px-6 py-4">
                      <div className="flex flex-col">
                        <span className="text-sm font-bold text-white tracking-tight">{formatSats(order.amount)}</span>
                        <span className="text-[10px] font-mono text-on-surface-variant/60">Fee: {formatSats(order.fee)}</span>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <span className={cn(
                        "text-[9px] font-bold font-mono uppercase tracking-widest px-2.5 py-1 rounded-full border",
                        order.state === 'COMPLETED' ? "text-secondary border-secondary/20 bg-secondary/5" :
                        order.state === 'FAILED' ? "text-error border-error/20 bg-error/5" :
                        order.state === 'PAID' ? "text-primary border-primary/20 bg-primary/5" :
                        "text-tertiary border-tertiary/20 bg-tertiary/5"
                      )}>
                        {order.state}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-right">
                       <div className="flex items-center justify-end gap-3 opacity-0 group-hover:opacity-100 transition-all translate-x-2 group-hover:translate-x-0">
                          <button title="View in Explorer" className="p-2 text-on-surface-variant hover:text-white hover:bg-white/10 rounded-lg transition-all border border-transparent hover:border-white/10">
                             <ExternalLink className="w-4 h-4" />
                          </button>
                          <button title="Options" className="p-2 text-on-surface-variant hover:text-white hover:bg-white/10 rounded-lg transition-all border border-transparent hover:border-white/10">
                             <MoreHorizontal className="w-4 h-4" />
                          </button>
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
          <div className="p-12 text-center border border-dashed border-white/5 rounded-2xl">
            <p className="text-xs font-mono text-on-surface-variant uppercase">No data found</p>
          </div>
        ) : (
          filteredOrders.map((order, i) => (
            <GlassCard 
              key={i} 
              onClick={() => onViewOrder(order.order_id)}
              className="p-5 border-white/5 cursor-pointer active:scale-[0.98] transition-transform"
            >
              <div className="flex justify-between items-start mb-4">
                <div className="flex items-center gap-3">
                  <span className="text-[10px] font-mono font-bold text-primary italic">LSPS1</span>
                  <span className="text-xs font-mono text-white">{order.order_id}</span>
                </div>
                <span className={cn(
                  "text-[9px] font-bold font-mono uppercase tracking-widest px-2 py-0.5 rounded-full border",
                  order.state === 'COMPLETED' ? "text-secondary border-secondary/20 bg-secondary/5" :
                  order.state === 'FAILED' ? "text-error border-error/20 bg-error/5" :
                  "text-primary border-primary/20 bg-primary/5"
                )}>
                  {order.state}
                </span>
              </div>
              
              <div className="grid grid-cols-2 gap-4 mb-4">
                <div>
                  <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest mb-1">Liquidity</p>
                  <p className="text-sm font-bold text-white">{formatSats(order.amount)}</p>
                </div>
                <div>
                  <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest mb-1">Node</p>
                  <p className="text-xs font-mono text-white truncate">{order.peer_id?.substring(0, 12)}...</p>
                </div>
              </div>
              
              <div className="pt-4 border-t border-white/5 flex justify-between items-center">
                 <p className="text-[10px] font-mono text-on-surface-variant">
                   {new Date(order.created_at * 1000).toLocaleString()}
                 </p>
                 <div className="flex gap-2">
                   <button className="p-2 text-on-surface-variant bg-white/5 rounded-lg">
                     <ExternalLink className="w-4 h-4" />
                   </button>
                   <button className="p-2 text-on-surface-variant bg-white/5 rounded-lg">
                     <MoreHorizontal className="w-4 h-4" />
                   </button>
                 </div>
              </div>
            </GlassCard>
          ))
        )}
      </div>
    </div>
  );
};
