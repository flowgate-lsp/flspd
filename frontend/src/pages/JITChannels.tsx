import React, { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Zap, Clock, CheckCircle2, ArrowUpRight, Hash, Database } from 'lucide-react';
import { useDashboard } from '../hooks/useDashboard';

interface JitChannel {
  order_id: string;
  peer_id: string;
  amount: number;
  state: string;
  funding_txid?: string;
  created_at: string;
}

export const JITChannels: React.FC = () => {
  const { api } = useDashboard();
  const [channels, setChannels] = useState<JitChannel[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchChannels = async () => {
      try {
        const data = await api.listJitChannels();
        setChannels(data);
      } catch (error) {
        console.error('Failed to fetch JIT channels:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchChannels();
  }, [api]);

  return (
    <div className="p-4 md:p-10 max-w-7xl mx-auto space-y-6 animate-in fade-in duration-700">
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-2">
            <Zap className="w-6 h-6 text-yellow-400" />
            JIT Channels
          </h1>
          <p className="text-slate-400">Track Just-In-Time liquidity deployments</p>
        </div>
        
        <div className="flex items-center gap-2 bg-slate-900/50 border border-slate-800 rounded-xl px-4 py-2">
          <Database className="w-4 h-4 text-indigo-400" />
          <span className="text-sm font-medium text-slate-300">{channels.length} Total Openings</span>
        </div>
      </div>

      {loading ? (
        <div className="space-y-3">
          {[1, 2, 3].map(i => (
            <div key={i} className="h-16 bg-slate-900/40 animate-pulse rounded-xl border border-slate-800/50" />
          ))}
        </div>
      ) : channels.length === 0 ? (
        <div className="bg-slate-900/40 border border-slate-800/50 rounded-2xl p-12 text-center">
          <Zap className="w-12 h-12 text-slate-700 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-white">No JIT channels active</h3>
          <p className="text-slate-400">JIT channels appear when users pay for liquidity on-demand</p>
        </div>
      ) : (
        <div className="bg-slate-900/40 border border-slate-800/50 rounded-2xl overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="bg-slate-800/30">
                  <th className="px-6 py-4 text-xs font-bold text-slate-400 uppercase tracking-widest">Order ID</th>
                  <th className="px-6 py-4 text-xs font-bold text-slate-400 uppercase tracking-widest">Peer</th>
                  <th className="px-6 py-4 text-xs font-bold text-slate-400 uppercase tracking-widest">Capacity</th>
                  <th className="px-6 py-4 text-xs font-bold text-slate-400 uppercase tracking-widest">Status</th>
                  <th className="px-6 py-4 text-xs font-bold text-slate-400 uppercase tracking-widest">Date</th>
                  <th className="px-6 py-4 text-xs font-bold text-slate-400 uppercase tracking-widest">TXID</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/50">
                {channels.map((ch, idx) => (
                  <motion.tr
                    key={ch.order_id}
                    initial={{ opacity: 0, x: -10 }}
                    animate={{ opacity: 1, x: 0 }}
                    transition={{ delay: idx * 0.03 }}
                    className="hover:bg-slate-800/20 transition-colors group"
                  >
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        <Hash className="w-3.5 h-3.5 text-slate-500" />
                        <span className="text-sm font-semibold text-white">{ch.order_id?.substring(0, 12)}...</span>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <span className="text-xs font-mono text-slate-400" title={ch.peer_id}>
                        {ch.peer_id?.substring(0, 12)}...
                      </span>
                    </td>
                    <td className="px-6 py-4">
                      <div className="text-sm font-medium text-slate-300">
                        {((ch.amount || 0) / 1e5).toLocaleString()} FLC
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <div className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] font-bold uppercase tracking-widest ${
                        ch.state === 'COMPLETED' ? 'bg-emerald-500/10 text-emerald-400' :
                        ch.state === 'PAID' ? 'bg-indigo-500/10 text-indigo-400' :
                        'bg-slate-500/10 text-slate-400'
                      }`}>
                        {ch.state === 'COMPLETED' ? <CheckCircle2 className="w-3 h-3" /> :
                         ch.state === 'PAID' ? <Zap className="w-3 h-3" /> :
                         <Clock className="w-3 h-3" />}
                        {ch.state}
                      </div>
                    </td>
                    <td className="px-6 py-4 text-sm text-slate-400">
                      {new Date(ch.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-6 py-4">
                      {ch.funding_txid ? (
                        <a 
                          href={`https://lokichain.info/tx/${ch.funding_txid}`} 
                          target="_blank" 
                          rel="noopener noreferrer"
                          className="flex items-center gap-1.5 text-indigo-400 hover:text-indigo-300 transition-colors text-xs group"
                        >
                          <span className="font-mono">{ch.funding_txid.substring(0, 8)}</span>
                          <ArrowUpRight className="w-3 h-3 group-hover:translate-x-0.5 group-hover:-translate-y-0.5 transition-transform" />
                        </a>
                      ) : (
                        <span className="text-xs text-slate-600 font-medium italic">Pending...</span>
                      )}
                    </td>
                  </motion.tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
};
