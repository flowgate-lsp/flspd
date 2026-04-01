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
    <div className="p-6 md:p-8 max-w-7xl mx-auto space-y-8 animate-in fade-in slide-in-from-bottom-2 duration-500">
      {/* Header Section */}
      <header className="flex flex-col md:flex-row md:items-center justify-between gap-6 mb-4">
        <div className="space-y-2">
          <h1 className="text-4xl md:text-5xl font-black italic tracking-tighter text-white uppercase leading-none">
            Command Center
          </h1>
          <p className="text-on-surface-variant font-mono text-[10px] tracking-[0.4em] uppercase font-bold opacity-80">
            Real-time Node Telemetry & Orchestration
          </p>
        </div>
        
        <GlassCard className="p-0 border-white/5 bg-surface-container-lowest/50" glow={false}>
          <div className="flex items-center gap-4 px-6 py-4">
            <div className="p-3 bg-primary/10 rounded-xl border border-primary/20">
              <Shield className="w-5 h-5 text-primary" />
            </div>
            <div>
              <p className="text-[9px] font-mono font-bold text-on-surface-variant uppercase tracking-[0.2em] leading-none mb-2 flex items-center gap-2">
                Node Protocol
                <span className="w-1.5 h-1.5 rounded-full bg-secondary animate-glow-pulse" />
              </p>
              <div className="flex items-center gap-3">
                <span className="text-sm font-bold text-white font-mono leading-none tracking-tight">
                  {summary ? middleTrim(summary.node_pubkey, 8) : 'Synchronizing Node...'}
                </span>
                {summary?.node_pubkey && (
                  <button 
                    onClick={() => copyToClipboard(summary.node_pubkey)}
                    className="p-1.5 hover:bg-white/5 rounded-lg transition-all active:scale-90"
                    title="Copy Pubkey"
                  >
                    {copied ? <Check className="w-3.5 h-3.5 text-secondary" /> : <Copy className="w-3.5 h-3.5 text-on-surface-variant hover:text-white" />}
                  </button>
                )}
              </div>
            </div>
          </div>
        </GlassCard>
      </header>

      {/* Hero Performance Stats */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatCard 
          label="24h Revenue" 
          value={formatSmart(summary?.revenue_24h || 0)} 
          icon={ArrowUpRight} 
          trend={summary?.revenue_trend || "0%"}
          color="text-secondary" 
          description="Gross earnings (Loki)"
        />
        <StatCard 
          label="Trading Volume" 
          value={formatSmart(summary?.liquidity_total || 0)} 
          icon={Activity} 
          trend={summary?.volume_trend || "0%"}
          color="text-primary" 
          description="Total node capacity"
        />
        <StatCard 
          label="Inbound Free" 
          value={formatSmart(summary?.liquidity_free || 0)} 
          icon={Zap} 
          color="text-tertiary" 
          description="Available liquidity"
        />
        <StatCard 
          label="Blocked Liquidity" 
          value={formatSmart(summary?.liquidity_blocked || 0)} 
          icon={Clock} 
          color="text-on-surface-variant" 
          description="Pending channel opens"
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-10 items-start">
        {/* Network & Infrastructure Card */}
        <GlassCard className="lg:col-span-2 p-6 space-y-6 overflow-hidden relative">
          <div className="flex items-center justify-between pb-4">
            <h2 className="text-sm font-bold text-white uppercase tracking-[0.2em] flex items-center gap-3">
              <Network className="w-4 h-4 text-primary" /> Infrastructure Health
            </h2>
          </div>
          
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-6">
            <MiniStatusCard 
               icon={Smartphone} 
               label="Uptime Score" 
               tag="Client Ops"
               tagColor="text-tertiary"
               value={`${summary?.uptime_score?.toFixed(1) || '0.0'}%`}
            />
            <MiniStatusCard 
               icon={Server} 
               label="Global Pool" 
               tag="Inbound"
               tagColor="text-primary"
               value={formatSmart(summary?.liquidity_total || 0).split(' ')[0]}
               subValue={formatSmart(summary?.liquidity_total || 0).split(' ')[1]}
            />
            <MiniStatusCard 
               icon={Hash} 
               label="Chain Head" 
               tag="Sync Level"
               tagColor="text-secondary"
               value={summary?.sync_height ? new Intl.NumberFormat().format(summary.sync_height) : '---,---'}
               isMono
            />
          </div>
        </GlassCard>

        {/* Audit Journal / Activity Feed */}
        <GlassCard className="p-6 space-y-6 h-full">
          <div className="flex items-center justify-between pb-4">
            <h2 className="text-sm font-bold text-white uppercase tracking-[0.2em]">Audit Journal</h2>
            <div className="p-2 rounded-lg bg-white/5">
              <Clock className="w-3.5 h-3.5 text-on-surface-variant" />
            </div>
          </div>
          
          <div className="space-y-2">
            {recentOrders.length > 0 ? recentOrders.map((activity) => (
              <div 
                key={activity.id} 
                className="flex items-center gap-4 p-3 rounded-2xl hover:bg-white/5 transition-all duration-300 group cursor-pointer border border-transparent hover:border-white/5"
                onClick={() => onViewOrder?.(activity.id)}
              >
                <div className={cn(
                  "w-12 h-12 rounded-xl flex items-center justify-center shrink-0 border border-white/5 bg-black/40 transition-all duration-500 shadow-lg group-hover:scale-110",
                  activity.status === 'success' ? "text-secondary shadow-secondary/5" : 
                  activity.status === 'failed' ? "text-error shadow-error/5" : "text-tertiary shadow-tertiary/5"
                )}>
                  {activity.type === 'order' && <ListIcon className="w-4 h-4" />}
                </div>
                <div className="flex-1 min-w-0">
                  <h4 className="text-[11px] font-bold text-white truncate mb-1 group-hover:text-primary transition-colors">{activity.title}</h4>
                  <div className="flex items-center gap-2">
                    <p className="text-[9px] font-mono text-on-surface-variant/60 uppercase tracking-tighter">{activity.time}</p>
                    <span className="w-1 h-1 rounded-full bg-white/10" />
                    <p className="text-[10px] font-mono text-on-surface-variant uppercase font-black italic tracking-tighter">{formatSmart(activity.amount)}</p>
                  </div>
                </div>
                <ChevronRight className="w-4 h-4 text-on-surface-variant/20 group-hover:text-primary transition-all group-hover:translate-x-1" />
              </div>
            )) : (
              <div className="flex flex-col items-center justify-center py-10 text-center space-y-3">
                 <div className="p-3 bg-white/5 rounded-full">
                    <Activity className="w-4 h-4 text-on-surface-variant/20" />
                 </div>
                 <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest">No recent entries</p>
              </div>
            )}
          </div>
          
        </GlassCard>
      </div>
    </div>
  );
};

/* Sub-components & Polish */

const StatCard: React.FC<{
  label: string;
  value: string | number;
  icon: any;
  trend?: string;
  color: string;
  description?: string;
}> = ({ label, value, icon: Icon, trend, color, description }) => (
  <GlassCard className="p-6 flex flex-col justify-between gap-6 group hover:border-white/20 transition-all duration-500 hover:bg-white/[0.02]">
    <div className="flex justify-between items-start leading-none">
      <div className={cn("p-3 rounded-2xl bg-black/40 border border-white/5 group-hover:scale-110 group-hover:rotate-3 transition-transform duration-500", color)}>
        <Icon className="w-5 h-5" />
      </div>
      {trend && (
        <span className={cn(
          "text-[10px] font-black italic bg-white/5 px-2.5 py-1 rounded-lg tracking-tighter shadow-sm", 
          trend.startsWith('+') ? "text-secondary" : trend === "0%" ? "text-on-surface-variant/50" : "text-error"
        )}>
          {trend}
        </span>
      )}
    </div>
    <div>
      <div className="space-y-1 mb-2">
        <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.2em] font-black flex items-center gap-2 leading-none">
          {label} 
          <span className="opacity-0 group-hover:opacity-100 transition-opacity duration-700">
             <Info className="w-2.5 h-2.5" />
          </span>
        </p>
        <p className="text-[9px] font-mono text-on-surface-variant/40 uppercase tracking-widest leading-none font-bold italic truncate">
          {description}
        </p>
      </div>
      <p className="text-3xl md:text-4xl font-black text-white italic tracking-tighter leading-none">{value}</p>
    </div>
  </GlassCard>
);

const MiniStatusCard: React.FC<{
  icon: any;
  label: string;
  tag: string;
  tagColor: string;
  value: string;
  subValue?: string;
  isMono?: boolean;
}> = ({ icon: Icon, label, tag, tagColor, value, subValue, isMono }) => (
  <div className="bg-black/40 border border-white/5 rounded-2xl p-6 flex flex-col justify-between h-40 hover:bg-white/[0.05] transition-all duration-500 group">
    <div className="flex justify-between items-start">
      <div className="p-2.5 bg-black/40 rounded-xl border border-white/5 text-on-surface-variant group-hover:text-white transition-colors">
        <Icon className="w-5 h-5" />
      </div>
      <span className={cn("text-[9px] font-black italic uppercase tracking-[0.1em]", tagColor)}>{tag}</span>
    </div>
    <div className="space-y-1">
      <div className="flex items-baseline gap-1">
        <p className={cn("text-2xl font-black text-white italic tracking-tighter", isMono && "font-mono")}>{value}</p>
        {subValue && <span className="text-xs font-mono text-on-surface-variant/50 uppercase font-bold italic">{subValue}</span>}
      </div>
      <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest leading-none font-bold">{label}</p>
    </div>
  </div>
);

const ListIcon: React.FC<{ className?: string }> = ({ className }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" className={className}>
    <line x1="8" y1="6" x2="21" y2="6"></line>
    <line x1="8" y1="12" x2="21" y2="12"></line>
    <line x1="8" y1="18" x2="21" y2="18"></line>
    <line x1="3" y1="6" x2="3.05" y2="6"></line>
    <line x1="3" y1="12" x2="3.05" y2="12"></line>
    <line x1="3" y1="18" x2="3.05" y2="18"></line>
  </svg>
);
