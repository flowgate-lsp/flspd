import React, { useState, useEffect } from 'react';
import { GlassCard } from '../components/GlassCard';
import { 
  BarChart, 
  Bar, 
  XAxis, 
  YAxis, 
  CartesianGrid, 
  Tooltip, 
  ResponsiveContainer,
  AreaChart,
  Area,
  LineChart,
  Line
} from 'recharts';
import { formatSmart } from '../lib/utils';
import { useDashboard } from '../hooks/useDashboard';
import { TrendingUp, Users, Zap, Award, Loader2 } from 'lucide-react';

export const Analytics: React.FC = () => {
  const { api, summary } = useDashboard();
  const [data, setData] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [stats, setStats] = useState({
    totalRevenue: 0,
    activePeers: 0,
    successRate: 0,
    avgOrderSize: 0
  });

  useEffect(() => {
    const fetchData = async () => {
      try {
        const analytics = await api.getAnalytics();
        setData(analytics.revenue_history || []);
        
        // Calculate summary stats from summary data if available
        if (summary) {
          const orders = await api.listOrders();
          const successOrders = orders.filter((o: any) => (o.state || o.OrderState) === 'COMPLETED');
          const rate = orders.length > 0 ? (successOrders.length / orders.length) * 100 : 0;
          const avg = successOrders.length > 0 ? (summary.revenue_24h || 0) / successOrders.length : 0;

          setStats({
            totalRevenue: summary.revenue_24h || 0,
            activePeers: summary.active_peers_count || 0,
            successRate: rate,
            avgOrderSize: avg
          });
        }
      } catch (err) {
        console.error('Failed to fetch analytics data:', err);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, [api, summary]);

  if (loading) {
    return (
      <div className="h-[60vh] flex items-center justify-center">
        <Loader2 className="w-8 h-8 text-primary animate-spin" />
      </div>
    );
  }

  return (
    <div className="p-4 md:p-10 max-w-7xl mx-auto space-y-8 animate-in fade-in duration-1000">
      <header>
        <h1 className="text-3xl font-bold text-white mb-2 uppercase italic tracking-tighter">Liquidity Radar</h1>
        <p className="text-on-surface-variant font-mono text-xs uppercase tracking-widest">Performance & Market Analytics</p>
      </header>

      {/* Analytics Summary */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <SummaryCard 
          label="Gross Revenue" 
          value={formatSmart(stats.totalRevenue)} 
          icon={TrendingUp} 
          color="text-secondary" 
        />
        <SummaryCard 
          label="Active Peers" 
          value={stats.activePeers} 
          icon={Users} 
          color="text-primary" 
        />
        <SummaryCard 
          label="Success Rate" 
          value={`${stats.successRate.toFixed(1)}%`} 
          icon={Award} 
          color="text-tertiary" 
        />
        <SummaryCard 
          label="Avg Order" 
          value={formatSmart(stats.avgOrderSize)} 
          icon={Zap} 
          color="text-secondary" 
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        <GlassCard className="h-[400px] flex flex-col">
          <h3 className="text-sm font-bold text-white uppercase tracking-widest mb-6">Revenue Timeline</h3>
          <div className="flex-1 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={data}>
                <defs>
                  <linearGradient id="colorRev" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#00BAFF" stopOpacity={0.3}/>
                    <stop offset="95%" stopColor="#00BAFF" stopOpacity={0}/>
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="#ffffff05" vertical={false} />
                <XAxis 
                  dataKey="date" 
                  stroke="#94a3b840" 
                  fontSize={10} 
                  tickLine={false} 
                  axisLine={false}
                  tickFormatter={(val) => new Date(val).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}
                />
                <YAxis 
                  stroke="#94a3b840" 
                  fontSize={10} 
                  tickLine={false} 
                  axisLine={false}
                  tickFormatter={(val) => formatSmart(val).split(' ')[0]}
                />
                <Tooltip 
                  contentStyle={{ backgroundColor: '#0f172a', border: '1px solid #ffffff10', borderRadius: '12px' }}
                  itemStyle={{ color: '#00BAFF', fontSize: '12px', fontWeight: 'bold' }}
                  labelStyle={{ color: '#94a3b8', fontSize: '10px', marginBottom: '4px' }}
                  formatter={(value: any) => [formatSmart(Number(value) || 0), 'Revenue']}
                />
                <Area type="monotone" dataKey="revenue" stroke="#00BAFF" strokeWidth={2} fillOpacity={1} fill="url(#colorRev)" />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </GlassCard>

        <GlassCard className="h-[400px] flex flex-col">
          <h3 className="text-sm font-bold text-white uppercase tracking-widest mb-6">Order Volume</h3>
          <div className="flex-1 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={data}>
                <CartesianGrid strokeDasharray="3 3" stroke="#ffffff05" vertical={false} />
                <XAxis 
                  dataKey="day" 
                  stroke="#94a3b840" 
                  fontSize={10} 
                  tickLine={false} 
                  axisLine={false}
                />
                <YAxis 
                  stroke="#94a3b840" 
                  fontSize={10} 
                  tickLine={false} 
                  axisLine={false} 
                />
                <Tooltip 
                  cursor={{ fill: '#ffffff05' }}
                  contentStyle={{ backgroundColor: '#0f172a', border: '1px solid #ffffff10', borderRadius: '12px' }}
                  itemStyle={{ color: '#00FD87', fontSize: '12px', fontWeight: 'bold' }}
                  labelStyle={{ color: '#94a3b8', fontSize: '10px', marginBottom: '4px' }}
                />
                <Bar dataKey="count" fill="#00FD87" radius={[4, 4, 0, 0]} barSize={20} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </GlassCard>

        <GlassCard className="lg:col-span-2 h-[400px] flex flex-col">
          <h3 className="text-sm font-bold text-white uppercase tracking-widest mb-6">Volume per Order Size</h3>
          <div className="flex-1 w-full">
             <ResponsiveContainer width="100%" height="100%">
                <LineChart data={data}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#ffffff05" vertical={false} />
                  <XAxis 
                    dataKey="day" 
                    stroke="#94a3b840" 
                    fontSize={10} 
                    tickLine={false} 
                    axisLine={false}
                  />
                  <YAxis 
                    stroke="#94a3b840" 
                    fontSize={10} 
                    tickLine={false} 
                    axisLine={false}
                    tickFormatter={(val) => formatSmart(val).split(' ')[0]}
                  />
                  <Tooltip 
                    contentStyle={{ backgroundColor: '#0f172a', border: '1px solid #ffffff10', borderRadius: '12px' }}
                    itemStyle={{ color: '#FFD700', fontSize: '12px', fontWeight: 'bold' }}
                    labelStyle={{ color: '#94a3b8', fontSize: '10px', marginBottom: '4px' }}
                    formatter={(value: any) => [formatSmart(Number(value) || 0), 'Volume']}
                  />
                  <Line type="monotone" dataKey="volume" stroke="#FFD700" strokeWidth={3} dot={{ fill: '#FFD700', r: 4 }} activeDot={{ r: 6, stroke: '#0f172a', strokeWidth: 2 }} />
                </LineChart>
             </ResponsiveContainer>
          </div>
        </GlassCard>
      </div>
    </div>
  );
};

const SummaryCard: React.FC<{ label: string; value: string | number; icon: any; color: string }> = ({ label, value, icon: Icon, color }) => (
  <GlassCard className="p-6 flex items-center gap-6">
    <div className={`p-3 rounded-2xl bg-black/40 border border-white/5 ${color}`}>
      <Icon className="w-6 h-6" />
    </div>
    <div>
      <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest mb-1">{label}</p>
      <p className="text-2xl font-black text-white italic tracking-tighter leading-none">{value}</p>
    </div>
  </GlassCard>
);
