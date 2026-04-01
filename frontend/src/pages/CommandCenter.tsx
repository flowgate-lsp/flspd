import React, { useState } from 'react';
import { GlassCard } from '../components/GlassCard';
import { 
  Activity, 
  ArrowUpRight, 
  Zap, 
  Shield, 
  Hash, 
  Clock, 
  ChevronRight, 
  Info, 
  Copy, 
  Check,
  Smartphone,
  Server,
  Network
} from 'lucide-react';
import { formatSmart, cn } from '../lib/utils';
import { useDashboard } from '../hooks/useDashboard';


export const CommandCenter: React.FC<{ onViewOrder?: (id: string) => void }> = ({ onViewOrder }) => {
  const { summary, orders } = useDashboard();
  const [copied, setCopied] = useState(false);

  const recentOrders = orders.slice(0, 5).map(o => ({
    id: o.order_id || o.OrderId,
    type: 'order' as const,
    title: `Order #${(o.order_id || o.OrderId || '').substring(0, 8)}`,
    amount: o.amount || o.OrderTotalLoki,
    time: new Date(o.created_at || o.CreatedAt).toLocaleTimeString(),
    status: (o.state || o.OrderState) === 'COMPLETED' ? 'success' as const : (o.state || o.OrderState) === 'FAILED' ? 'failed' as const : 'pending' as const
  }));

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const middleTrim = (str: string, layout: number = 6) => {
    if (!str) return '...';
    if (str.length <= layout * 2) return str;
    return `${str.substring(0, layout)}...${str.substring(str.length - layout)}`;
  };

  return (
    <div className="p-4 md:p-10 max-w-7xl mx-auto space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-1000">
      <header className="flex flex-col md:flex-row md:items-center justify-between gap-6">
        <div className="space-y-1">
          <h1 className="text-4xl font-black italic tracking-tighter text-white uppercase leading-none">
            Command Center
          </h1>
          <p className="text-on-surface-variant font-mono text-xs tracking-[0.2em] uppercase">Real-time Node Telemetry</p>
        </div>
        
        <div className="flex items-center gap-3 bg-surface-container-lowest border border-white/5 rounded-2xl p-4 pr-6">
          <div className="p-2.5 bg-primary/10 rounded-xl">
            <Shield className="w-5 h-5 text-primary" />
          </div>
          <div>
            <p className="text-[10px] font-mono font-bold text-on-surface-variant uppercase tracking-widest leading-none mb-1.5 flex items-center gap-2">
              Node Protocol
              <span className="w-1.5 h-1.5 rounded-full bg-secondary animate-glow-pulse" />
            </p>
            <div className="flex items-center gap-2">
              <span className="text-sm font-bold text-white font-mono leading-none">
                {summary ? middleTrim(summary.node_pubkey) : 'Loading...'}
              </span>
              {summary?.node_pubkey && (
                <button 
                  onClick={() => copyToClipboard(summary.node_pubkey)}
                  className="p-1 hover:bg-white/5 rounded transition-colors"
                  title="Copy Pubkey"
                >
                  {copied ? <Check className="w-3 h-3 text-secondary" /> : <Copy className="w-3 h-3 text-on-surface-variant" />}
                </button>
              )}
            </div>
          </div>
        </div>
      </header>

      {/* Hero Stats */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatCard 
          label="24h Revenue" 
          value={formatSmart(summary?.revenue_24h || 0)} 
          icon={ArrowUpRight} 
          trend="+12.5%" 
          color="text-secondary" 
        />
        <StatCard 
          label="Trading Volume" 
          value={formatSmart(summary?.liquidity_total || 0)} 
          icon={Activity} 
          trend="+4.2%" 
          color="text-primary" 
        />
        <StatCard 
          label="Inbound Free" 
          value={formatSmart(summary?.liquidity_free || 0)} 
          icon={Zap} 
          color="text-tertiary" 
        />
        <StatCard 
          label="Pending Liquidity" 
          value={formatSmart(summary?.liquidity_blocked || 0)} 
          icon={Clock} 
          color="text-on-surface-variant" 
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* Network Topology */}
        <GlassCard className="lg:col-span-2 space-y-6 overflow-hidden relative">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-bold text-white uppercase tracking-widest flex items-center gap-2">
              <Network className="w-4 h-4 text-primary" /> Network Status
            </h2>
            <button className="text-[10px] font-bold text-on-surface-variant hover:text-white transition-colors uppercase tracking-widest">Global View</button>
          </div>
          
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="bg-black/30 border border-white/5 rounded-2xl p-5 flex flex-col justify-between h-32">
              <div className="flex justify-between items-start">
                <Smartphone className="w-5 h-5 text-on-surface-variant" />
                <span className="text-[10px] font-mono text-tertiary">Client Ops</span>
              </div>
              <div>
                <p className="text-2xl font-black text-white italic tracking-tighter">98.2<span className="text-xs font-mono ml-1 text-on-surface-variant/50 uppercase">%</span></p>
                <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest">Uptime Score</p>
              </div>
            </div>
            <div className="bg-black/30 border border-white/5 rounded-2xl p-5 flex flex-col justify-between h-32">
              <div className="flex justify-between items-start">
                <Server className="w-5 h-5 text-on-surface-variant" />
                <span className="text-[10px] font-mono text-primary">Inbound Capacity</span>
              </div>
              <div>
                <p className="text-2xl font-black text-white italic tracking-tighter"> {formatSmart(summary?.liquidity_total || 0)} </p>
                <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest">Global Pool</p>
              </div>
            </div>
            <div className="bg-black/30 border border-white/5 rounded-2xl p-5 flex flex-col justify-between h-32">
              <div className="flex justify-between items-start">
                <Hash className="w-5 h-5 text-on-surface-variant" />
                <span className="text-[10px] font-mono text-secondary">Sync Height</span>
              </div>
              <div>
                 <p className="text-2xl font-black text-white italic tracking-tighter">842,109</p>
                 <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest">Chain Head</p>
              </div>
            </div>
          </div>
        </GlassCard>

        {/* Activity Feed */}
        <GlassCard className="space-y-6">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-bold text-white uppercase tracking-widest">Journal</h2>
            <Clock className="w-4 h-4 text-on-surface-variant" />
          </div>
          <div className="space-y-4">
            {recentOrders.map((activity) => (
              <div 
                key={activity.id} 
                className="flex items-center gap-4 group cursor-pointer"
                onClick={() => onViewOrder?.(activity.id)}
              >
                <div className={cn(
                  "w-10 h-10 rounded-xl flex items-center justify-center shrink-0 border border-white/5 bg-black/40 transition-all group-hover:border-primary/30",
                  activity.status === 'success' ? "text-secondary" : 
                  activity.status === 'failed' ? "text-error" : "text-tertiary"
                )}>
                  {activity.type === 'order' && <ListIcon className="w-4 h-4" />}
                </div>
                <div className="flex-1 min-w-0">
                  <h4 className="text-xs font-bold text-white truncate">{activity.title}</h4>
                  <div className="flex items-center gap-2">
                    <p className="text-[10px] font-mono text-on-surface-variant uppercase">{activity.time}</p>
                    <span className="w-1 h-1 rounded-full bg-white/10" />
                    <p className="text-[10px] font-mono text-on-surface-variant uppercase font-bold">{formatSmart(activity.amount)}</p>
                  </div>
                </div>
                <ChevronRight className="w-4 h-4 text-on-surface-variant/30 group-hover:text-primary transition-all group-hover:translate-x-1" />
              </div>
            ))}
          </div>
          <button className="w-full py-3 rounded-xl border border-white/5 text-[10px] font-mono font-bold uppercase tracking-widest text-on-surface-variant hover:text-white hover:bg-white/5 transition-all">
            Full Audit Log
          </button>
        </GlassCard>
      </div>
    </div>
  );
};

/* Sub-components */

const StatCard: React.FC<{
  label: string;
  value: string | number;
  icon: any;
  trend?: string;
  color: string;
}> = ({ label, value, icon: Icon, trend, color }) => (
  <GlassCard className="p-5 flex flex-col justify-between gap-4 group hover:border-white/10 transition-all h-full">
    <div className="flex justify-between items-start">
      <div className={cn("p-2 rounded-xl bg-black/30 border border-white/5 group-hover:scale-110 transition-transform", color)}>
        <Icon className="w-5 h-5" />
      </div>
      {trend && (
        <span className={cn("text-[10px] font-mono font-bold bg-white/5 px-2 py-0.5 rounded italic", trend.startsWith('+') ? "text-secondary" : "text-error")}>
          {trend}
        </span>
      )}
    </div>
    <div>
      <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest mb-1.5 flex items-center gap-1.5">
        {label} <Info className="w-2.5 h-2.5 opacity-0 group-hover:opacity-30 transition-opacity" />
      </p>
      <p className="text-3xl font-black text-white italic tracking-tighter leading-none">{value}</p>
    </div>
  </GlassCard>
);

const ListIcon: React.FC<{ className?: string }> = ({ className }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className={className}>
    <line x1="8" y1="6" x2="21" y2="6"></line>
    <line x1="8" y1="12" x2="21" y2="12"></line>
    <line x1="8" y1="18" x2="21" y2="18"></line>
    <line x1="3" y1="6" x2="3.01" y2="6"></line>
    <line x1="3" y1="12" x2="3.01" y2="12"></line>
    <line x1="3" y1="18" x2="3.01" y2="18"></line>
  </svg>
);
