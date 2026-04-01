import React, { useState, useEffect, useMemo } from 'react';
import { GlassCard } from '../components/GlassCard';
import { ArrowLeft, Clock, Zap, Hash, ExternalLink, Copy, CheckCircle, AlertCircle, Loader2 } from 'lucide-react';
import { formatLoki, cn } from '../lib/utils';
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
  ORDER_CREATED: 'text-on-surface-variant border-on-surface-variant/20 bg-on-surface-variant/5',
  PAYMENT_RECEIVED: 'text-primary border-primary/20 bg-primary/5',
  CHANNEL_FUNDED: 'text-tertiary border-tertiary/20 bg-tertiary/5',
  ORDER_COMPLETED: 'text-secondary border-secondary/20 bg-secondary/5',
};

export const OrderDetail: React.FC<OrderDetailProps> = ({ orderId, onBack }) => {
  const { getAuthHeader } = useAuth();
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

  const api = useMemo(() => {
    return new DashboardAPI(window.location.origin, getAuthHeader);
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

  if (loading) {
    return (
      <div className="h-[60vh] flex items-center justify-center">
        <Loader2 className="w-8 h-8 text-primary animate-spin" />
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="p-4 md:p-10 max-w-5xl mx-auto">
        <button onClick={onBack} className="flex items-center gap-2 text-on-surface-variant hover:text-white text-xs font-mono uppercase tracking-widest mb-8 transition-colors">
          <ArrowLeft className="w-4 h-4" /> Back to Matrix
        </button>
        <div className="h-[40vh] flex flex-col items-center justify-center gap-4">
          <AlertCircle className="w-12 h-12 text-error" />
          <p className="text-on-surface-variant font-mono text-xs">{error || 'Order not found'}</p>
        </div>
      </div>
    );
  }

  const order = data.order;
  const timeline = data.timeline || [];

  const stateColor = order.state === 'COMPLETED' ? 'text-secondary' :
                     order.state === 'FAILED' ? 'text-error' : 'text-primary';

  return (
    <div className="p-4 md:p-10 max-w-5xl mx-auto space-y-8 animate-in fade-in duration-700">
      {/* Back + Header */}
      <div>
        <button onClick={onBack} className="flex items-center gap-2 text-on-surface-variant hover:text-white text-xs font-mono uppercase tracking-widest mb-6 transition-colors group">
          <ArrowLeft className="w-4 h-4 group-hover:-translate-x-1 transition-transform" /> Back to Matrix
        </button>
        <div className="flex flex-col md:flex-row md:items-end md:justify-between gap-4">
          <div>
            <h1 className="text-3xl md:text-4xl font-bold tracking-tighter text-white uppercase italic leading-none mb-2">
              Order Inspector
            </h1>
            <p className="text-on-surface-variant font-mono text-xs tracking-widest uppercase flex items-center gap-2">
              <span className="text-[10px] border border-primary/20 bg-primary/5 text-primary px-2 py-0.5 rounded italic font-bold">LSPS1</span>
              {orderId}
            </p>
          </div>
          <span className={cn("text-sm font-mono font-bold uppercase tracking-widest", stateColor)}>
            {order.state}
          </span>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Main Details */}
        <GlassCard className="lg:col-span-2 space-y-6">
          <h2 className="text-sm font-bold text-white uppercase tracking-widest">Order Parameters</h2>

          <div className="grid grid-cols-2 gap-6">
            <DetailRow label="LSP Balance" value={formatLoki(order.lsp_balance_loki)} />
            <DetailRow label="Client Balance" value={formatLoki(order.client_balance_loki)} />
            <DetailRow label="Order Total" value={formatLoki(order.amount)} highlight />
            <DetailRow label="Fee Total" value={formatLoki(order.fee)} />
            <DetailRow label="Funding Confirms" value={`${order.funding_confirms_within_blocks} blocks`} />
            <DetailRow label="Channel Expiry" value={`${order.channel_expiry_blocks} blocks`} />
            <DetailRow label="Announce Channel" value={order.announce_channel ? 'Yes' : 'No'} />
            <DetailRow label="Payment State" value={order.payment_state} />
          </div>

          {/* Copyable Fields */}
          <div className="space-y-3 pt-4 border-t border-white/5">
            <CopyableField
              label="Peer ID"
              value={order.peer_id}
              copied={copied}
              onCopy={copyToClipboard}
            />
            {order.funding_txid && (
              <CopyableField
                label="Funding TX"
                value={order.funding_txid}
                copied={copied}
                onCopy={copyToClipboard}
                explorerUrl={`https://explorer.lokichain.org/tx/${order.funding_txid}`}
              />
            )}
            {order.refund_onchain_address && (
              <CopyableField
                label="Refund Address"
                value={order.refund_onchain_address}
                copied={copied}
                onCopy={copyToClipboard}
              />
            )}
          </div>
        </GlassCard>

        {/* Timeline */}
        <GlassCard className="lg:col-span-1">
          <h2 className="text-sm font-bold text-white uppercase tracking-widest mb-6">Timeline</h2>
          <div className="space-y-0">
            {timeline.map((event: any, i: number) => {
              const Icon = timelineIcons[event.event] || Clock;
              const colorClass = timelineColors[event.event] || 'text-on-surface-variant';
              const isLast = i === timeline.length - 1;

              return (
                <div key={i} className="flex gap-4">
                  {/* Vertical line + dot */}
                  <div className="flex flex-col items-center">
                    <div className={cn("w-8 h-8 rounded-full border flex items-center justify-center shrink-0", colorClass)}>
                      <Icon className="w-4 h-4" />
                    </div>
                    {!isLast && <div className="w-px h-full min-h-[40px] bg-white/10" />}
                  </div>
                  {/* Content */}
                  <div className="pb-6">
                    <p className="text-xs font-mono font-bold text-white uppercase tracking-wider">{event.event.replace(/_/g, ' ')}</p>
                    <p className="text-[10px] font-mono text-on-surface-variant mt-1">{event.detail}</p>
                    <p className="text-[10px] font-mono text-on-surface-variant/50 mt-0.5 italic">
                      {new Date(event.timestamp).toLocaleString()}
                    </p>
                  </div>
                </div>
              );
            })}
          </div>
        </GlassCard>
      </div>
    </div>
  );
};

/* Sub-components */

const DetailRow: React.FC<{ label: string; value: string; highlight?: boolean }> = ({ label, value, highlight }) => (
  <div>
    <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest mb-1">{label}</p>
    <p className={cn("text-sm font-bold", highlight ? "text-primary" : "text-white")}>{value}</p>
  </div>
);

const CopyableField: React.FC<{
  label: string;
  value: string;
  copied: string | null;
  onCopy: (text: string, label: string) => void;
  explorerUrl?: string;
}> = ({ label, value, copied, onCopy, explorerUrl }) => (
  <div>
    <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest mb-1">{label}</p>
    <div className="flex items-center gap-2 p-3 rounded-xl bg-black/30 border border-white/5 group">
      <code className="text-xs font-mono text-white/80 truncate flex-1">{value}</code>
      <button
        onClick={() => onCopy(value, label)}
        className="p-1.5 text-on-surface-variant hover:text-white rounded-lg hover:bg-white/5 transition-all shrink-0"
        title="Copy"
      >
        {copied === label ? <CheckCircle className="w-3.5 h-3.5 text-secondary" /> : <Copy className="w-3.5 h-3.5" />}
      </button>
      {explorerUrl && (
        <a
          href={explorerUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="p-1.5 text-on-surface-variant hover:text-primary rounded-lg hover:bg-white/5 transition-all shrink-0"
          title="View in Explorer"
        >
          <ExternalLink className="w-3.5 h-3.5" />
        </a>
      )}
    </div>
  </div>
);
