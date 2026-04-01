import React, { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Users, Globe, ArrowUpRight, ArrowDownLeft, Zap, Search, ShieldCheck, Activity } from 'lucide-react';
import { useDashboard } from '../hooks/useDashboard';

interface Peer {
  pub_key: string;
  address: string;
  inbound: boolean;
  active: boolean;
  in_sat: number;
  out_sat: number;
}

export const PeerNodes: React.FC = () => {
  const { api } = useDashboard();
  const [peers, setPeers] = useState<Peer[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');

  useEffect(() => {
    const fetchPeers = async () => {
      try {
        const data = await api.listPeers();
        setPeers(data);
      } catch (error) {
        console.error('Failed to fetch peers:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchPeers();
  }, [api]);

  const filteredPeers = peers.filter(p => 
    (p.pub_key?.toLowerCase() || '').includes(search.toLowerCase()) || 
    (p.address?.toLowerCase() || '').includes(search.toLowerCase())
  );

  return (
    <div className="p-4 md:p-10 max-w-7xl mx-auto space-y-6 animate-in fade-in duration-700">
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-2">
            <Users className="w-6 h-6 text-indigo-400" />
            Peer Nodes
          </h1>
          <p className="text-slate-400">Manage your Lightning Network peer connections</p>
        </div>

        <div className="relative group">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-500 group-focus-within:text-indigo-400 transition-colors" />
          <input
            type="text"
            placeholder="Search by pubkey or address..."
            className="bg-slate-900/50 border border-slate-800 rounded-xl py-2 pl-10 pr-4 text-white focus:outline-none focus:ring-2 focus:ring-indigo-500/50 w-full md:w-64 transition-all"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
      </div>

      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3].map(i => (
            <div key={i} className="h-48 bg-slate-900/40 animate-pulse rounded-2xl border border-slate-800/50" />
          ))}
        </div>
      ) : filteredPeers.length === 0 ? (
        <div className="bg-slate-900/40 border border-slate-800/50 rounded-2xl p-12 text-center">
          <Users className="w-12 h-12 text-slate-700 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-white">No peers found</h3>
          <p className="text-slate-400">Try adjusting your search or check your node connection</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filteredPeers.map((peer, idx) => (
            <motion.div
              key={peer.pub_key}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.05 }}
              className="bg-slate-900/40 border border-slate-800/50 rounded-2xl p-5 hover:border-indigo-500/30 hover:bg-slate-900/60 transition-all group"
            >
              <div className="flex items-start justify-between mb-4">
                <div className="p-2.5 bg-indigo-500/10 rounded-xl">
                  <Globe className="w-5 h-5 text-indigo-400" />
                </div>
                <div className={`flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium ${
                  peer.active ? 'bg-emerald-500/10 text-emerald-400' : 'bg-rose-500/10 text-rose-400'
                }`}>
                  <Activity className="w-3 h-3" />
                  {peer.active ? 'Online' : 'Offline'}
                </div>
              </div>

              <div>
                <h3 className="text-sm font-semibold text-white mb-1 flex items-center gap-1.5 leading-none">
                  Peer #{peer.pub_key?.substring(0, 8)}
                  {peer.inbound && <ShieldCheck className="w-3.5 h-3.5 text-slate-500" />}
                </h3>
                <p className="text-xs text-slate-500 font-mono truncate mb-4">
                  {peer.pub_key}
                </p>
              </div>

              <div className="grid grid-cols-2 gap-3 pb-4 border-b border-slate-800/50 mb-4">
                <div>
                  <label className="text-[10px] uppercase tracking-wider font-bold text-slate-500 mb-1 block">Sent</label>
                  <div className="flex items-center gap-1 text-slate-300">
                    <ArrowUpRight className="w-3 h-3 text-emerald-400" />
                    <span className="text-sm font-medium">{(peer.in_sat / 1e5).toFixed(2)} FLC</span>
                  </div>
                </div>
                <div>
                  <label className="text-[10px] uppercase tracking-wider font-bold text-slate-500 mb-1 block">Received</label>
                  <div className="flex items-center gap-1 text-slate-300">
                    <ArrowDownLeft className="w-3 h-3 text-indigo-400" />
                    <span className="text-sm font-medium">{(peer.out_sat / 1e5).toFixed(2)} FLC</span>
                  </div>
                </div>
              </div>

              <div className="flex items-center justify-between">
                <div className="text-[10px] text-slate-500 truncate max-w-[140px]" title={peer.address}>
                  {peer.address || 'No address info'}
                </div>
                <button className="text-[10px] font-bold text-indigo-400 hover:text-indigo-300 transition-colors uppercase tracking-widest flex items-center gap-1">
                  Connect <Zap className="w-3 h-3" />
                </button>
              </div>
            </motion.div>
          ))}
        </div>
      )}
    </div>
  );
};
