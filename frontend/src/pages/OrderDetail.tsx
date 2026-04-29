import React, { useState, useEffect, useMemo } from 'react';
import { GlassCard } from '../components/GlassCard';
import { ArrowLeft, Clock, Zap, Hash, ExternalLink, Copy, CheckCircle, AlertCircle, Loader2, Activity, CornerUpRight } from 'lucide-react';
import { useFormatAmount, cn } from '../lib/utils';
import { useAuth } from '../hooks/useAuth';
import { DashboardAPI } from '../lib/api';

interface OrderDetailProps {
  orderId: string;
  onBack: () => void;
}

const timelineIcons: Record<string, typeof CheckCircle> = {
  ORDER_CREATED: Clock,
  PAYMENT_RECEIVED: Zap,
  CHANNEL_FUNDED: Hash,
  ORDER_COMPLETED: CheckCircle,
};

const timelineColors: Record<string, string> = {
  ORDER_CREATED: 'text-on-surface-variant border-on-surface-variant/20 bg-on-surface-variant/5 shadow-on-surface-variant/5',
  PAYMENT_RECEIVED: 'text-primary border-primary/20 bg-primary/5 shadow-primary/5',
  CHANNEL_FUNDED: 'text-tertiary border-tertiary/20 bg-tertiary/5 shadow-tertiary/5',
  ORDER_COMPLETED: 'text-secondary border-secondary/20 bg-secondary/5 shadow-secondary/5',
};

export const OrderDetail: React.FC<OrderDetailProps> = ({ orderId, onBack }) => {
  const { getAuthHeader } = useAuth();
  const { format } = useFormatAmount();
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

  const api = useMemo(() => {
    const proxyUrl = (import.meta as any).env?.VITE_PROXY_URL;
    return new DashboardAPI(window.location.origin, getAuthHeader, proxyUrl);
  }, [getAuthHeader]);

  useEffect(() => {
    setLoading(true);
    api.getOrderDetail(orderId)
      .then(setData)
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, [orderId, api]);

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text);
    setCopied(label);
    setTimeout(() => setCopied(null), 2000);
  };

  const middleTrim = (str: string, layout: number = 8) => {
    if (!str) return '...';
    if (str.length <= layout * 2) return str;
    return `${str.substring(0, layout)}...${str.substring(str.length - layout)}`;
  };

  if (loading) {
    return (
      <div className="h-[60vh] flex items-center justify-center">
        <div className="flex flex-col items-center gap-6">
          <Loader2 className="w-12 h-12 text-primary animate-spin" />
          <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.4em] font-black animate-pulse italic">Reconstructing Order Context...</p>
        </div>
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="p-6 md:p-10 max-w-5xl mx-auto space-y-12">
        <button onClick={onBack} className="flex items-center gap-3 text-on-surface-variant hover:text-white text-[11px] font-black uppercase tracking-[0.2em] mb-8 transition-all group italic">
           <ArrowLeft className="w-4 h-4 group-hover:-translate-x-1 transition-transform" /> Back to Matrix
        </button>
        <div className="h-[40vh] flex flex-col items-center justify-center gap-6 text-center">
          <div className="p-4 rounded-full bg-error/5 border border-error/20">
            <AlertCircle className="w-12 h-12 text-error animate-pulse" />
          </div>
          <h2 className="text-2xl font-black text-white uppercase italic tracking-tighter">Order Resolution Failed</h2>
          <p className="text-on-surface-variant font-mono text-[10px] uppercase tracking-widest max-w-sm opacity-60 italic">{error || 'Order identifier invalid or expired'}</p>
        </div>
      </div>
    );
  }

  const order = data.order;
  const timeline = data.timeline || [];

  const stateColor = order.state === 'COMPLETED' ? 'text-secondary border-secondary/20 bg-secondary/5' :
                     order.state === 'FAILED' ? 'text-error border-error/20 bg-error/5' : 
                     'text-primary border-primary/20 bg-primary/5';

  return (
    <div className="p-6 md:p-10 max-w-6xl mx-auto space-y-12 animate-in fade-in slide-in-from-bottom-2 duration-500">
      {/* Back + Header */}
      <div className="space-y-4">
        <button onClick={onBack} className="flex items-center gap-3 text-on-surface-variant hover:text-white text-[11px] font-black uppercase tracking-[0.2em] transition-all group italic">
          <ArrowLeft className="w-4 h-4 group-hover:-translate-x-1 transition-transform" /> Retruncate Matrix
        </button>
        <div className="flex flex-col md:flex-row md:items-end md:justify-between gap-8 pb-8">
          <div className="space-y-4">
            <h1 className="text-4xl md:text-5xl font-black tracking-tighter text-white uppercase italic leading-none">
              Order Inspector
            </h1>
            <div className="flex items-center flex-wrap gap-4">
              <span className="text-[10px] font-mono font-black italic text-primary border border-primary/30 bg-primary/10 px-3 py-1 rounded-xl uppercase tracking-widest">
                LSPS1 Protocol
              </span>
              <span className="text-[11px] font-mono text-white/50 tracking-tighter font-bold select-all flex items-center gap-2">
                <Hash className="w-3.5 h-3.5" /> {orderId}
              </span>
            </div>
          </div>
          
          <div className={cn("px-6 py-3 rounded-2xl border text-xs font-black italic uppercase tracking-[0.2em] shadow-xl flex items-center gap-4 transition-all duration-700", stateColor)}>
             <div className="w-2 h-2 rounded-full bg-current animate-pulse shadow-[0_0_10px_currentColor]" />
             {order.state}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-10 items-start">
        {/* Main Details Panel */}
        <GlassCard className="lg:col-span-2 p-10 space-y-12 shadow-2xl relative overflow-hidden group">
          <div className="absolute top-0 right-0 w-64 h-64 bg-primary/5 blur-[120px] rounded-full opacity-30 group-hover:opacity-60 transition-opacity" />
          
          <div className="flex items-center gap-3 pb-6">
            <Activity className="w-4 h-4 text-primary" />
            <h2 className="text-sm font-black text-white uppercase tracking-[0.2em]">Operational Parameters</h2>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-y-10 gap-x-12 relative z-10">
            <DetailRow label="Inbound Liquidity" value={format(order.lsp_balance_loki)} icon={CornerUpRight} color="text-secondary" />
            <DetailRow label="Client Reserve" value={format(order.client_balance_loki)} />
            <DetailRow label="Settlement Total" value={format(order.amount)} highlight />
            <DetailRow label="Network Fee" value={format(order.fee, 'MLOKI')} />
            <DetailRow label="Inbound Window" value={`${order.funding_confirms_within_blocks} Blocks`} />
            <DetailRow label="Channel TTL" value={`${order.channel_expiry_blocks} Blocks`} />
            <DetailRow label="Public Announce" value={order.announce_channel ? 'ENABLED' : 'DISABLED'} color={order.announce_channel ? 'text-secondary' : 'text-error'} />
            <DetailRow label="Payment State" value={order.payment_state} color="text-tertiary" />
          </div>

          {/* Persistent Cryptographic Identifiers */}
          <div className="space-y-6 pt-10 relative z-10">
            <h3 className="text-[10px] font-black text-on-surface-variant uppercase tracking-[0.4em] mb-4">Cryptographic Identity</h3>
            <div className="space-y-4">
              <div className="flex items-center gap-3">
                <p className="text-[9px] font-mono text-on-surface-variant/40 uppercase tracking-[0.3em] font-black italic leading-none">Peer Identity</p>
                {order.peer_alias && (
                  <span 
                    className="text-[10px] font-mono font-black uppercase italic px-2 py-0.5 rounded border border-current/20 bg-current/5"
                    style={{ color: order.peer_color || '#00baff' }}
                  >
                    {order.peer_alias}
                  </span>
                )}
              </div>
              <CopyableField
                label=""
                value={order.peer_id}
                copied={copied}
                onCopy={copyToClipboard}
              />
            </div>
            {order.funding_txid && (
              <CopyableField
                label="Funding Transmission Hash"
                value={order.funding_txid}
                copied={copied}
                onCopy={copyToClipboard}
                explorerUrl={`https://lokichain.info/tx/${order.funding_txid}`}
              />
            )}
            {order.refund_onchain_address && (
              <CopyableField
                label="Refund Settlement Address"
                value={order.refund_onchain_address}
                copied={copied}
                onCopy={copyToClipboard}
              />
            )}
          </div>
        </GlassCard>

        {/* Temporal Sequence / Timeline */}
        <GlassCard className="lg:col-span-1 p-10 space-y-10 h-full flex flex-col shadow-2xl">
          <div className="flex items-center justify-between pb-6">
            <h2 className="text-sm font-black text-white uppercase tracking-[0.2em]">Sequence</h2>
            <Clock className="w-4 h-4 text-on-surface-variant/30" />
          </div>
          
          <div className="space-y-0 relative flex-1">
            {/* Timeline Line */}
            <div className="absolute left-[19px] top-4 bottom-4 w-[2px] bg-white/5 shadow-inner" />
            
            {timeline.map((event: any, i: number) => {
              const Icon = timelineIcons[event.event] || Clock;
              const colorClass = timelineColors[event.event] || 'text-on-surface-variant border-on-surface-variant/20 bg-on-surface-variant/5';
              const isLast = i === timeline.length - 1;

              // Extract Funding TX link if present
              const txMatch = event.detail?.match(/Funding TX: ([a-f0-9]{64})/);

              return (
                <div key={i} className="flex gap-8 relative group">
                  {/* Icon Node */}
                  <div className="flex flex-col items-center relative z-10">
                    <div className={cn("w-10 h-10 rounded-2xl border flex items-center justify-center shrink-0 transition-all duration-500 shadow-lg group-hover:scale-110", colorClass)}>
                      <Icon className="w-4 h-4" />
                    </div>
                  </div>
                  
                  {/* Event Detail */}
                  <div className={cn("pb-10 transition-all duration-500", isLast ? "pb-0" : "")}>
                    <p className="text-[11px] font-black text-white uppercase tracking-widest italic group-hover:text-primary transition-colors leading-none mb-2">
                       {event.event.replace(/_/g, ' ')}
                    </p>
                    
                    {txMatch ? (
                      <div className="bg-black/40 border border-white/5 p-2 rounded-lg mt-2 inline-flex items-center gap-2 group/txLink hover:bg-white/10 transition-all">
                        <span className="text-[9px] font-mono text-on-surface-variant uppercase font-black tracking-tight">TX: </span>
                        <a 
                          href={`https://lokichain.info/tx/${txMatch[1]}`}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-[9px] font-mono text-primary font-black underline decoration-primary/20 hover:decoration-primary underline-offset-2 transition-all italic"
                        >
                          {middleTrim(txMatch[1], 4)}
                        </a>
                      </div>
                    ) : (
                      <p className="text-[10px] font-mono text-on-surface-variant/60 mt-1 leading-relaxed max-w-[200px]">{event.detail}</p>
                    )}
                    
                    <p className="text-[9px] font-mono text-on-surface-variant/30 mt-2 italic font-black uppercase tracking-widest">
                      {new Date(event.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                    </p>
                  </div>
                </div>
              );
            })}
          </div>
          
          <div className="pt-8 mt-auto">
             <p className="text-[9px] font-mono text-on-surface-variant/20 uppercase font-black text-center tracking-[0.2em]">Verified Secure Chain Record</p>
          </div>
        </GlassCard>
      </div>
    </div>
  );
};

/* Sub-components & Polish */

const DetailRow: React.FC<{ label: string; value: string; highlight?: boolean; icon?: any; color?: string }> = ({ label, value, highlight, icon: Icon, color }) => (
  <div className="group/row">
    <div className="flex items-center gap-2 mb-2">
      {Icon && <Icon className={cn("w-3 h-3 opacity-30 group-hover/row:opacity-100 transition-opacity", color || 'text-primary')} />}
      <p className="text-[9px] font-mono text-on-surface-variant uppercase tracking-[0.2em] font-black italic">{label}</p>
    </div>
    <p className={cn("text-lg md:text-xl font-black italic tracking-tighter leading-none transition-all duration-300 group-hover/row:translate-x-1", highlight ? "text-primary drop-shadow-[0_0_10px_rgba(0,186,255,0.2)]" : color || "text-white")}>{value}</p>
  </div>
);

const CopyableField: React.FC<{
  label: string;
  value: string;
  copied: string | null;
  onCopy: (text: string, label: string) => void;
  explorerUrl?: string;
}> = ({ label, value, copied, onCopy, explorerUrl }) => (
  <div className="group/field">
    <p className="text-[9px] font-mono text-on-surface-variant/40 uppercase tracking-[0.3em] mb-2 font-black italic leading-none">{label}</p>
    <div className="flex items-center gap-3 p-4 rounded-2xl bg-black/40 border border-white/10 group-focus-within:border-primary/40 transition-all duration-500 hover:bg-white/[0.03]">
      <code className="text-[11px] font-mono text-white/70 truncate flex-1 font-bold group-hover/field:text-white transition-colors">{value}</code>
      <div className="flex items-center gap-2 border-l border-white/10 pl-3">
        <button
          onClick={() => onCopy(value, label)}
          className="p-2 text-on-surface-variant hover:text-white rounded-xl hover:bg-white/10 transition-all shrink-0 active:scale-90"
          title="Copy"
        >
          {copied === label ? <CheckCircle className="w-3.5 h-3.5 text-secondary" /> : <Copy className="w-3.5 h-3.5" />}
        </button>
        {explorerUrl && (
          <a
            href={explorerUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="p-2 text-on-surface-variant hover:text-primary rounded-xl hover:bg-white/10 transition-all shrink-0 active:scale-90"
            title="View in Explorer"
          >
            <ExternalLink className="w-3.5 h-3.5" />
          </a>
        )}
      </div>
    </div>
  </div>
);
