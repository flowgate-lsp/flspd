import React, { useState } from 'react';
import { GlassCard } from '../components/GlassCard';
import {
  Activity,
  ArrowUpRight,
  ArrowDownLeft,
  Shield,
  Hash,
  Clock,
  Info,
  Copy,
  Check,
  Wallet,
  Users,
  Link2,
  Network
} from 'lucide-react';
import { useFormatAmount, cn } from '../lib/utils';
import { useDashboard } from '../hooks/useDashboard';
import { useNavigate } from '@tanstack/react-router';

export const CommandCenter: React.FC<{ onViewOrder?: (id: string) => void }> = ({ onViewOrder }) => {
  const { summary, orders } = useDashboard();
  const { format } = useFormatAmount();
  const [copied, setCopied] = useState(false);
  const navigate = useNavigate();

  const priorityStates = new Set(['PAID', 'FAILED']);
  const priorityOrders = orders.filter(o => priorityStates.has(o.state || o.OrderState));
  const hasPriorityOrders = priorityOrders.length > 0;
  const sourceOrders = (hasPriorityOrders ? priorityOrders : orders).slice(0, 5);

  const recentOrders = sourceOrders.map(o => ({
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

  const connectionString = summary?.node_pubkey
    ? summary.node_host
      ? `${summary.node_pubkey}@${summary.node_host}`
      : summary.node_pubkey
    : '';

  const nodeStatus = summary?.node_status; // 'online' | 'syncing' | 'unreachable'
  const statusLabel = nodeStatus === 'online' ? 'Synced' : nodeStatus === 'syncing' ? 'Syncing' : 'Unreachable';

  return (
    <div className="p-3 md:p-8 max-w-7xl mx-auto space-y-8 animate-in fade-in slide-in-from-bottom-2 duration-500">
      {/* 1. Hero Identity Strip */}
      <GlassCard className="p-6 md:p-8 border border-outline-variant bg-surface-container-high/40 overflow-hidden relative group">
        <div className="absolute -top-32 -left-32 w-96 h-96 bg-primary/10 blur-[120px] rounded-full pointer-events-none opacity-40 group-hover:opacity-70 transition-opacity duration-1000" />
        <div className="relative z-10 flex flex-col xl:flex-row xl:items-start justify-between gap-8 w-full">
          <div className="space-y-3 max-w-xl">
            <h1 className="text-4xl md:text-5xl font-black italic tracking-tighter text-white uppercase leading-none shadow-sm">
              Command Center
            </h1>
            <p className="text-on-surface-variant font-mono text-[10px] tracking-[0.4em] uppercase font-black opacity-60">
              Real-time Telemetry & Orchestration Array
            </p>
          </div>

          {/* Integrated Node Protocol Plaque */}
          <div className="p-4 rounded-xl bg-surface-container-high/70 border border-outline-variant w-full xl:w-auto min-w-[320px] flex flex-col gap-3 backdrop-blur-md shadow-2xl relative">
            <div className="flex items-center justify-between w-full">
              <p className="text-[9px] font-mono text-on-surface-variant uppercase tracking-[0.2em] font-black flex items-center gap-2">
                <Shield className="w-3.5 h-3.5 text-primary" />
                Node Protocol {summary?.node_name && <span className="text-primary/60 italic font-bold">[{summary.node_name}]</span>}
                <span className="w-1 h-1 rounded-full bg-secondary shadow-[0_0_8px_rgba(0,253,135,0.8)] animate-pulse ml-1" />
              </p>
              {summary?.node_pubkey && (
                <button
                  onClick={() => copyToClipboard(connectionString)}
                  className="p-1 hover:bg-white/5 text-white/30 hover:text-white rounded transition-colors active:scale-95"
                  title="Copy Connection String"
                >
                  {copied ? <Check className="w-3.5 h-3.5 text-secondary" /> : <Copy className="w-3.5 h-3.5" />}
                </button>
              )}
            </div>
            <div className="font-mono text-[11px] leading-relaxed text-white/80 break-all select-text pb-1">
               {summary ? (connectionString || <span className="text-white/20 italic">No listen address configured</span>) : <span className="text-white/20 italic">Synchronizing Core Engine...</span>}
            </div>
          </div>
        </div>
      </GlassCard>

      {/* Bento Grid Application */}
      <div className="grid grid-cols-1 lg:grid-cols-4 gap-8">
        
        {/* Left Span: Financials & Infrastructure (3 columns wide) */}
        <div className="lg:col-span-3 space-y-8 flex flex-col h-full">
          
          {/* 2. Headline Financial Grid */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
            <StatCard
              label="24h Revenue"
              value={format(summary?.revenue_24h || 0)}
              icon={ArrowUpRight}
              trend={summary?.revenue_trend || "0%"}
              color="text-secondary"
              description="Gross earnings"
            />
            <StatCard
              label="On-Chain Balance"
              value={format(summary?.liquidity_free || 0)}
              icon={Wallet}
              color="text-primary"
              description="Spendable wallet funds"
            />
          </div>

          {/* 3. Liquidity Posture */}
          <GlassCard className="p-0 border border-outline-variant bg-surface-container-high/40 overflow-hidden relative flex flex-col">
            <div className="px-6 py-4 border-b border-outline-variant bg-white/[0.02]">
              <h2 className="text-[10px] font-black font-mono text-on-surface-variant uppercase tracking-[0.2em] flex items-center gap-2">
                <Activity className="w-3.5 h-3.5 text-primary" /> Liquidity Posture
              </h2>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 divide-y sm:divide-y-0 sm:divide-x divide-outline-variant w-full bg-surface-container-low/40">
              <MiniStatusCard
                 icon={ArrowUpRight}
                 label="Outbound Liquidity"
                 tag="Can Send"
                 tagColor="text-secondary"
                 value={format(summary?.liquidity_outbound || 0)}
              />
              <MiniStatusCard
                 icon={ArrowDownLeft}
                 label="Inbound Liquidity"
                 tag="Can Receive"
                 tagColor="text-primary"
                 value={format(summary?.liquidity_inbound || 0)}
              />
              <MiniStatusCard
                 icon={Clock}
                 label="Reserved"
                 tag="Pending Quotes"
                 tagColor="text-tertiary"
                 value={format(summary?.liquidity_blocked || 0)}
              />
            </div>
          </GlassCard>

          {/* 4. Node Health */}
          <GlassCard className="p-0 border border-outline-variant bg-surface-container-high/40 overflow-hidden flex-1 relative flex flex-col">
            <div className="px-6 py-4 border-b border-outline-variant bg-white/[0.02] flex items-center justify-between">
              <h2 className="text-[10px] font-black font-mono text-on-surface-variant uppercase tracking-[0.2em] flex items-center gap-2">
                <Network className="w-3.5 h-3.5 text-primary" /> Node Health
              </h2>
              <span className={cn(
                "flex items-center gap-2 text-[9px] font-mono uppercase font-black tracking-widest",
                nodeStatus === 'online' ? "text-secondary" : nodeStatus === 'syncing' ? "text-tertiary" : "text-error"
              )}>
                <span className={cn(
                  "w-1.5 h-1.5 rounded-full shadow-[0_0_8px_currentColor]",
                  nodeStatus === 'online' ? "bg-secondary animate-pulse" : nodeStatus === 'syncing' ? "bg-tertiary animate-pulse" : "bg-error"
                )} />
                {statusLabel}
              </span>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 divide-y sm:divide-y-0 sm:divide-x divide-outline-variant flex-1 w-full bg-surface-container-low/40">
              <MiniStatusCard
                 icon={Users}
                 label="Peers Connected"
                 tag="Network"
                 tagColor="text-secondary"
                 value={summary?.peers_connected != null ? String(summary.peers_connected) : '--'}
                 isMono
              />
              <MiniStatusCard
                 icon={Link2}
                 label="Active Channels"
                 tag={summary?.channels_pending ? `${summary.channels_pending} Pending` : "Live"}
                 tagColor={summary?.channels_pending ? "text-tertiary" : "text-primary"}
                 value={summary?.channels_active != null ? String(summary.channels_active) : '--'}
                 isMono
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
        </div>

        {/* 5. Attention Queue Vertical (Right Sidebar, 1 column wide) */}
        <GlassCard className="lg:col-span-1 p-0 border border-outline-variant bg-surface-container-high/40 flex flex-col h-full overflow-hidden">
          <div className="px-6 py-5 border-b border-outline-variant bg-white/[0.02] flex items-center justify-between shadow-sm z-10 relative">
            <h2 className="text-[10px] font-black font-mono text-on-surface-variant uppercase tracking-[0.2em]">Attention Queue</h2>
            {hasPriorityOrders ? (
              <span className="w-2 h-2 rounded-full bg-error shadow-[0_0_8px_rgba(255,50,50,0.6)] animate-pulse" />
            ) : (
              <Clock className="w-3.5 h-3.5 text-on-surface-variant/40" />
            )}
          </div>

          <div className="flex-1 overflow-y-auto px-4 py-4 space-y-1.5 custom-scrollbar bg-surface-container-low/40">
            {recentOrders.length > 0 ? recentOrders.map((activity) => (
              <div 
                key={activity.id} 
                className="flex items-center gap-4 p-3 rounded-xl hover:bg-white/[0.04] transition-all duration-300 group cursor-pointer border border-transparent hover:border-outline-variant"
                onClick={() => onViewOrder?.(activity.id)}
              >
                <div className={cn(
                  "w-10 h-10 rounded-lg flex items-center justify-center shrink-0 border border-outline-variant bg-surface-container-high/70 transition-transform duration-500 group-hover:scale-105",
                  activity.status === 'success' ? "text-secondary" : 
                  activity.status === 'failed' ? "text-error" : "text-tertiary"
                )}>
                  {activity.type === 'order' && <ListIcon className="w-3.5 h-3.5" />}
                </div>
                <div className="flex-1 min-w-0">
                  <h4 className="text-[10px] font-black font-mono text-white/90 truncate group-hover:text-primary transition-colors tracking-widest break-all mb-1">{activity.title}</h4>
                  <div className="flex items-center justify-between w-full">
                    <p className="text-[8px] font-mono text-white/30 uppercase tracking-[0.1em]">{activity.time}</p>
                    <p className="text-[9px] font-mono text-on-surface-variant uppercase font-black tracking-widest tabular-nums tabular-nums-variant">{format(activity.amount)}</p>
                  </div>
                </div>
              </div>
            )) : (
              <div className="flex flex-col items-center justify-center h-40 text-center space-y-3 opacity-50">
                 <Activity className="w-6 h-6 text-on-surface-variant/20" />
                 <p className="text-[9px] font-mono text-on-surface-variant uppercase tracking-[0.3em] font-black">No recent entries</p>
              </div>
            )}
          </div>
          
          <div className="px-5 py-3 border-t border-outline-variant bg-white/[0.02]">
            <button
              onClick={() => navigate({ to: '/orders' })}
              className="w-full text-center text-[9px] font-mono font-black text-white/40 hover:text-white uppercase tracking-[0.2em] transition-colors py-2 active:scale-95"
            >
              View Order Matrix
            </button>
          </div>
        </GlassCard>
      </div>
    </div>
  );
};

/* --- Refined Sub-components for Bento Box --- */

const StatCard: React.FC<{
  label: string;
  value: string | number;
  icon: any;
  trend?: string;
  color: string;
  description?: string;
}> = ({ label, value, icon: Icon, trend, color, description }) => {
  const parts = String(value).split(' ');
  const num = parts[0];
  const suffix = parts.slice(1).join(' ');

  return (
    <div className="p-6 rounded-2xl bg-surface-container-high/40 border border-outline-variant hover:border-outline transition-all duration-500 hover:bg-surface-container-high/70 group relative overflow-hidden flex flex-col justify-between">
      <div className="flex justify-between items-start mb-6">
        <div className="flex items-center gap-2.5">
            <div className={cn("w-1.5 h-1.5 rounded-full shadow-[0_0_8px_currentColor]", color.replace("text-", "bg-"))} />
            <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.2em] font-black flex items-center gap-2">
              {label}
              <span className="opacity-0 group-hover:opacity-100 transition-opacity duration-700 cursor-help" title={description}>
                 <Info className="w-3 h-3 text-white/30 hover:text-white/60" />
              </span>
            </p>
        </div>
        <Icon className={cn("w-4 h-4 opacity-40 group-hover:opacity-100 transition-opacity duration-500", color)} />
      </div>

      <div>
        <div className="flex items-baseline gap-2 mb-2">
          <p className="text-3xl font-black text-white tracking-tighter tabular-nums leading-[0.8]">{num}</p>
          {suffix && <span className="text-[10px] font-mono text-white/30 uppercase font-black tracking-widest">{suffix}</span>}
        </div>
        {trend && (
          <div className="flex items-center gap-1.5 pt-2">
            <span className={cn(
              "text-[9px] font-black font-mono px-2 py-0.5 rounded border tracking-widest uppercase",
              trend.startsWith('+') ? "text-secondary border-secondary/20 bg-secondary/5" : trend === "0%" ? "text-white/40 border-outline bg-white/5" : "text-error border-error/20 bg-error/5"
            )}>
              {trend}
            </span>
          </div>
        )}
      </div>
    </div>
  );
};

const MiniStatusCard: React.FC<{
  icon: any;
  label: string;
  tag: string;
  tagColor: string;
  value: string;
  isMono?: boolean;
}> = ({ icon: Icon, label, tag, tagColor, value, isMono }) => {
  const parts = String(value).split(' ');
  const num = parts[0];
  const suffix = parts.slice(1).join(' ');

  return (
    <div className="p-6 flex flex-col justify-between h-36 group hover:bg-white/[0.02] transition-colors relative">
      <div className="flex justify-between items-start">
        <Icon className="w-4 h-4 text-white/30 group-hover:text-white/60 transition-colors" />
        <span className={cn("text-[8px] font-black font-mono uppercase tracking-[0.2em]", tagColor)}>{tag}</span>
      </div>
      <div className="space-y-1 mt-auto">
        <div className="flex items-baseline gap-1.5">
          <p className={cn("text-2xl font-black text-white tracking-tighter tabular-nums leading-none", isMono && "font-mono")}>{num}</p>
          {suffix && <span className="text-[9px] font-mono text-white/30 uppercase font-black tracking-widest">{suffix}</span>}
        </div>
        <p className="text-[9px] font-mono text-on-surface-variant uppercase tracking-[0.2em] font-black">{label}</p>
      </div>
    </div>
  );
};

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
