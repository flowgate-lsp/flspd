import React from 'react';
import { GlassCard } from '../components/GlassCard';
import { Activity, TrendingUp, Zap, Clock, ChevronRight, Loader2, AlertCircle } from 'lucide-react';
import { formatLoki, cn } from '../lib/utils';
import { useDashboard } from '../hooks/useDashboard';
import { useEvents } from '../hooks/useEvents';

interface CommandCenterProps {
  onViewOrder: (id: string) => void;
}

export const CommandCenter: React.FC<CommandCenterProps> = ({ onViewOrder }) => {
  const { summary, orders, loading, error, refreshSummary, refreshOrders } = useDashboard();

  useEvents((event) => {
    if (event.type === 'order_state_changed') {
      refreshSummary();
      refreshOrders();
    }
  });

  if (loading && !summary) {
    return (
      <div className="h-[60vh] flex items-center justify-center">
        <Loader2 className="w-8 h-8 text-primary animate-spin" />
      </div>
    );
  }

  if (error && !summary) {
    return (
      <div className="h-[60vh] flex flex-col items-center justify-center gap-4 text-center px-6">
        <AlertCircle className="w-12 h-12 text-error animate-bounce" />
        <h2 className="text-xl font-bold text-white uppercase italic tracking-widest">Connection Interrupted</h2>
        <p className="text-on-surface-variant font-mono text-xs max-w-sm">{error}</p>
      </div>
    );
  }

  const nodeName = summary?.node_name || 'FLSPD NODE';
  const revenue = summary?.revenue_24h || 0;
  const liquidityTotal = summary?.liquidity_total || 0;
  const liquidityFree = summary?.liquidity_free || 0;
  const liquidityBlocked = summary?.liquidity_blocked || 0;
  const activeOrders = summary?.active_orders?.PAID || 0;

  return (
    <div className="p-4 md:p-10 max-w-7xl mx-auto space-y-8 md:space-y-10 animate-in fade-in duration-1000">
      {/* Header */}
      <header className="flex flex-col md:flex-row md:justify-between md:items-end gap-4">
        <div>
          <h1 className="text-3xl md:text-4xl font-bold tracking-tighter text-white mb-2 uppercase italic leading-none">Command Center</h1>
          <p className="text-on-surface-variant font-mono text-xs md:text-sm tracking-widest uppercase flex items-center gap-2">
            <span className="w-1.5 h-1.5 rounded-full bg-secondary animate-pulse" />
            System Operational // {nodeName}
          </p>
        </div>
        <div className="flex gap-4">
          <GlassCard className="py-2 px-4 flex items-center gap-3 border-secondary/20">
            <span className="w-2 h-2 rounded-full bg-secondary shadow-[0_0_8px_rgba(0,253,135,0.5)]" />
            <span className="text-[10px] font-mono font-bold uppercase text-secondary">LND Online</span>
          </GlassCard>
        </div>
      </header>

      {/* KPI Row */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 md:gap-6">
        <GlassCard delay={0.1} glow className="relative overflow-hidden group">
          <div className="flex justify-between items-start mb-4">
            <div className="p-2 rounded-lg bg-primary/10 group-hover:bg-primary/20 transition-colors">
              <TrendingUp className="w-5 h-5 text-primary" />
            </div>
          </div>
          <h3 className="text-[10px] font-mono uppercase tracking-widest mb-1 text-on-surface-variant">Total Capacity</h3>
          <p className="text-2xl font-bold text-white tracking-tight leading-none">
            {(liquidityTotal / 100000000).toFixed(2)} <span className="text-xs font-normal text-on-surface-variant italic">FLC</span>
          </p>
        </GlassCard>

        <GlassCard delay={0.2} glow className="group">
          <div className="flex justify-between items-start mb-4">
            <div className="p-2 rounded-lg bg-tertiary/10 group-hover:bg-tertiary/20 transition-colors">
              <Zap className="w-5 h-5 text-tertiary" />
            </div>
            <span className="text-tertiary font-mono text-[10px] px-2 py-0.5 rounded bg-tertiary/5 border border-tertiary/20">ACTIVE</span>
          </div>
          <h3 className="text-[10px] font-mono uppercase tracking-widest mb-1 text-on-surface-variant">Live Orders</h3>
          <p className="text-2xl font-bold text-white tracking-tight leading-none">
            {activeOrders} <span className="text-xs font-normal text-on-surface-variant italic">PAID</span>
          </p>
        </GlassCard>

        <GlassCard delay={0.3} className="group">
          <div className="flex justify-between items-start mb-4">
            <div className="p-2 rounded-lg bg-secondary/10 group-hover:bg-secondary/20 transition-colors">
              <Clock className="w-5 h-5 text-secondary" />
            </div>
          </div>
          <h3 className="text-[10px] font-mono uppercase tracking-widest mb-1 text-on-surface-variant">24h Revenue</h3>
          <p className="text-2xl font-bold text-white tracking-tight leading-none">{formatLoki(revenue)}</p>
        </GlassCard>

        <GlassCard delay={0.4} className="flex flex-col justify-center">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest">Available</span>
              <span className="text-[10px] font-mono text-white">{(liquidityFree / 100000000).toFixed(3)} FLC</span>
            </div>
            <div className="h-1.5 w-full bg-white/5 rounded-full overflow-hidden">
              <div 
                className="h-full bg-primary rounded-full shadow-[0_0_10px_rgba(0,186,255,0.5)] transition-all duration-1000" 
                style={{ width: `${(liquidityFree / liquidityTotal) * 100 || 0}%` }}
              />
            </div>
            <div className="flex items-center justify-between">
              <span className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest">Reserved</span>
              <span className="text-[10px] font-mono text-white">{(liquidityBlocked / 100000000).toFixed(3)} FLC</span>
            </div>
            <div className="h-1.5 w-full bg-white/5 rounded-full overflow-hidden">
              <div 
                className="h-full bg-secondary rounded-full shadow-[0_0_10px_rgba(0,253,135,0.5)] transition-all duration-1000" 
                style={{ width: `${(liquidityBlocked / liquidityTotal) * 100 || 0}%` }}
              />
            </div>
          </div>
        </GlassCard>
      </div>

      {/* Activity & Recent Feed */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <GlassCard delay={0.5} className="lg:col-span-2 min-h-[400px]">
          <div className="flex items-center justify-between mb-8">
            <h2 className="text-lg font-bold text-white flex items-center gap-3">
              Order Matrix
              <span className="text-[10px] font-mono border border-white/10 px-2 py-0.5 rounded text-on-surface-variant uppercase">LATEST</span>
            </h2>
            <button 
              onClick={() => onViewOrder('')} // Empty string or special case to just go to list
              className="text-[10px] font-mono uppercase tracking-widest text-primary flex items-center gap-1 hover:brightness-125 transition-all group"
            >
              Full Matrix <ChevronRight className="w-3 h-3 group-hover:translate-x-1 transition-transform" />
            </button>
          </div>
          
          <div className="space-y-1">
            {orders.length === 0 ? (
              <div className="h-40 flex items-center justify-center border border-dashed border-white/5 rounded-xl">
                 <p className="text-xs font-mono text-on-surface-variant uppercase">No recent transmission data</p>
              </div>
            ) : (
              orders.slice(0, 6).map((item, i) => (
                <div 
                  key={i} 
                  onClick={() => onViewOrder(item.order_id)}
                  className="flex items-center justify-between p-4 bg-white/0 hover:bg-white/5 rounded-xl transition-all group border border-transparent hover:border-white/5 cursor-pointer"
                >
                  <div className="flex items-center gap-4">
                    <div className={cn(
                      "w-1 h-8 rounded-full shadow-lg",
                      item.state === 'COMPLETED' ? "bg-secondary shadow-secondary/20" : 
                      item.state === 'FAILED' ? "bg-error shadow-error/20" : "bg-primary shadow-primary/20"
                    )} />
                    <div>
                      <div className="flex items-center gap-2 mb-0.5">
                        <span className="text-xs font-mono font-bold text-white uppercase tracking-tighter">LSPS1</span>
                        <span className="text-[10px] font-mono text-on-surface-variant uppercase tracking-tighter">{item.order_id}</span>
                      </div>
                      <p className="text-sm font-medium text-white">{formatLoki(item.amount)}</p>
                    </div>
                  </div>
                  <div className="text-right">
                    <span className={cn(
                      "text-[9px] font-bold font-mono uppercase tracking-widest px-2 py-0.5 rounded-full border",
                      item.state === 'COMPLETED' ? "text-secondary border-secondary/20 bg-secondary/5" :
                      item.state === 'FAILED' ? "text-error border-error/20 bg-error/5" :
                      "text-primary border-primary/20 bg-primary/5"
                    )}>
                      {item.state}
                    </span>
                    <p className="text-[10px] font-mono text-on-surface-variant mt-1 italic">
                      {new Date(item.created_at * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                    </p>
                  </div>
                </div>
              ))
             )}
          </div>
        </GlassCard>

        <GlassCard delay={0.6} className="lg:col-span-1">
          <div className="flex items-center justify-between mb-8">
            <h2 className="text-lg font-bold text-white uppercase italic tracking-tighter">Node Protocol</h2>
          </div>
          <div className="space-y-6">
             <div className="flex items-center gap-4">
                <div className="w-12 h-12 rounded-full bg-primary/5 flex items-center justify-center p-2 border border-primary/10">
                  <Activity className="w-6 h-6 text-primary" />
                </div>
                <div>
                  <h4 className="text-sm font-bold text-white">{nodeName}</h4>
                  <p className="text-[10px] font-mono text-on-surface-variant break-all opacity-60">
                    {summary?.node_pubkey?.substring(0, 32)}...
                  </p>
                </div>
             </div>
             
             <div className="p-5 rounded-2xl bg-black/40 border border-white/5 space-y-4">
                <div className="flex justify-between items-center group cursor-default">
                  <span className="text-[10px] font-mono uppercase text-on-surface-variant tracking-widest group-hover:text-white transition-colors">Version</span>
                  <span className="text-[10px] font-mono text-white font-bold">{summary?.version || 'v0.0.5'}</span>
                </div>
                <div className="flex justify-between items-center group cursor-default">
                  <span className="text-[10px] font-mono uppercase text-on-surface-variant tracking-widest group-hover:text-white transition-colors">LSP ID</span>
                  <span className="text-[10px] font-mono text-white">0001</span>
                </div>
                <div className="flex justify-between items-center group cursor-default">
                  <span className="text-[10px] font-mono uppercase text-on-surface-variant tracking-widest group-hover:text-white transition-colors">Compliance</span>
                  <span className="text-[10px] font-mono text-secondary font-bold">NIP-98/NIP-07</span>
                </div>
             </div>

             <div className="pt-4">
               <button className="w-full py-3 rounded-xl border border-primary/30 text-primary text-[10px] font-mono font-bold uppercase tracking-[0.2em] hover:bg-primary/10 hover:border-primary transition-all active:scale-95">
                 Synchronize State
               </button>
             </div>
          </div>
        </GlassCard>
      </div>
    </div>
  );
};
