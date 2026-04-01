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
  PieChart,
  Pie,
  Cell,
  Legend
} from 'recharts';
import { formatSmart } from '../lib/utils';
import { useDashboard } from '../hooks/useDashboard';
import { TrendingUp, Users, Zap, Award, Loader2, BarChart3, PieChart as PieIcon, Activity, CornerUpRight } from 'lucide-react';
import { cn } from '../lib/utils';

export const Analytics: React.FC = () => {
  const { api } = useDashboard();
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const analytics = await api.getAnalytics();
        setData(analytics);
      } catch (err) {
        console.error('Failed to fetch analytics data:', err);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, [api]);

  if (loading || !data) {
    return (
      <div className="h-[60vh] flex items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <Loader2 className="w-10 h-10 text-primary animate-spin" />
          <p className="text-on-surface-variant font-mono text-[10px] uppercase tracking-[0.4em] animate-pulse font-bold">Synchronizing Radar...</p>
        </div>
      </div>
    );
  }

  const { 
    total_revenue, 
    success_rate, 
    avg_order_size, 
    active_peers, 
    revenue_history, 
    order_distribution 
  } = data;

  return (
    <div className="p-6 md:p-10 max-w-7xl mx-auto space-y-12 animate-in fade-in slide-in-from-bottom-6 duration-1000">
      {/* Header Section */}
      <header className="flex flex-col md:flex-row md:items-end justify-between gap-8 pb-6 border-b border-white/5">
        <div className="space-y-2">
          <div className="flex items-center gap-4 mb-1">
            <div className="p-2.5 rounded-xl bg-primary/10 border border-primary/20 shadow-[0_0_15px_rgba(0,186,255,0.1)]">
              <Activity className="w-6 h-6 text-primary" />
            </div>
            <h1 className="text-4xl md:text-5xl font-black text-white uppercase italic tracking-tighter leading-none">
              Liquidity Radar
            </h1>
          </div>
          <p className="text-on-surface-variant font-mono text-[10px] uppercase tracking-[0.4em] font-black opacity-70">
            Real-time Node Performance & Market Intelligence
          </p>
        </div>
        <div className="flex items-center gap-6 text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.2em] font-black">
          <div className="flex items-center gap-2 bg-white/5 px-3 py-1.5 rounded-lg border border-white/5">
            <span className="w-2 h-2 rounded-full bg-secondary shadow-[0_0_10px_rgba(0,253,135,0.6)] animate-pulse" />
            Live Sync
          </div>
          <span className="hidden md:inline text-white/5">|</span>
          <div className="hidden md:flex items-center gap-2 text-white/40">
             <div className="p-1 rounded bg-white/5">
                <Users className="w-3 h-3 text-primary" />
             </div>
             Node Protocol: Active
          </div>
        </div>
      </header>

      {/* High-Level Metrics Summary */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <SummaryCard 
          label="Total Revenue" 
          value={formatSmart(total_revenue)} 
          icon={TrendingUp} 
          color="text-secondary" 
          description="Inbound fees earned"
          trend="+12.4% vs 7d"
        />
        <SummaryCard 
          label="Success Rate" 
          value={`${success_rate.toFixed(1)}%`} 
          icon={Award} 
          color="text-primary" 
          description="Channel funding success"
          trend="Optimal Flow"
        />
        <SummaryCard 
          label="Avg Order Size" 
          value={formatSmart(avg_order_size)} 
          icon={Zap} 
          color="text-tertiary" 
          description="Mean liquidity request"
        />
        <SummaryCard 
          label="Connected Peers" 
          value={active_peers} 
          icon={Users} 
          color="text-white" 
          description="Active node connections"
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-10 items-start">
        {/* Main Revenue Chart */}
        <GlassCard className="lg:col-span-2 p-8 flex flex-col h-[520px]">
          <div className="flex items-center justify-between mb-10 border-b border-white/5 pb-5">
            <div className="flex items-center gap-4">
              <div className="p-2 rounded-lg bg-primary/5">
                <BarChart3 className="w-4 h-4 text-primary" />
              </div>
              <h3 className="text-xs font-black text-white uppercase tracking-[0.2em]">Revenue & Order Velocity</h3>
            </div>
            <div className="flex gap-6">
               <div className="flex items-center gap-2 text-[10px] font-mono text-on-surface-variant font-bold uppercase tracking-widest">
                  <div className="w-2.5 h-2.5 rounded-full bg-primary shadow-[0_0_8px_rgba(0,186,255,0.4)]" /> Revenue
               </div>
               <div className="flex items-center gap-2 text-[10px] font-mono text-on-surface-variant font-bold uppercase tracking-widest">
                  <div className="w-2.5 h-2.5 rounded-full bg-secondary shadow-[0_0_8px_rgba(0,253,135,0.4)]" /> Volume
               </div>
            </div>
          </div>
          
          <div className="flex-1 w-full relative">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={revenue_history} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                <defs>
                  <linearGradient id="colorRevenue" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#00BAFF" stopOpacity={0.3}/>
                    <stop offset="95%" stopColor="#00BAFF" stopOpacity={0}/>
                  </linearGradient>
                  <linearGradient id="colorVolume" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#00FD87" stopOpacity={0.1}/>
                    <stop offset="95%" stopColor="#00FD87" stopOpacity={0}/>
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="#ffffff05" vertical={false} />
                <XAxis 
                  dataKey="day" 
                  stroke="#94a3b830" 
                  fontSize={10} 
                  tickLine={false} 
                  axisLine={false}
                  dy={10}
                  fontFamily="monospace"
                />
                <YAxis 
                  stroke="#94a3b830" 
                  fontSize={10} 
                  tickLine={false} 
                  axisLine={false}
                  tickFormatter={(val) => formatSmart(val).split(' ')[0]}
                  fontFamily="monospace"
                />
                <Tooltip 
                  content={<CustomTooltip />}
                  cursor={{ stroke: '#ffffff10', strokeWidth: 1 }}
                />
                <Area 
                  type="monotone" 
                  dataKey="revenue" 
                  stroke="#00BAFF" 
                  strokeWidth={3} 
                  fillOpacity={1} 
                  fill="url(#colorRevenue)" 
                  animationDuration={1500}
                />
                <Area 
                  type="monotone" 
                  dataKey="count" 
                  stroke="#00FD87" 
                  strokeWidth={2} 
                  strokeDasharray="4 4"
                  fillOpacity={1} 
                  fill="url(#colorVolume)" 
                  animationDuration={2000}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </GlassCard>

        {/* Order Composition (Pie) */}
        <GlassCard className="lg:col-span-1 p-8 flex flex-col h-[520px]">
          <div className="flex items-center gap-4 mb-10 border-b border-white/5 pb-5">
            <div className="p-2 rounded-lg bg-secondary/5">
              <PieIcon className="w-4 h-4 text-secondary" />
            </div>
            <h3 className="text-xs font-black text-white uppercase tracking-[0.2em]">Order Distribution</h3>
          </div>
          
          <div className="flex-1 w-full relative">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={order_distribution}
                  cx="50%"
                  cy="45%"
                  innerRadius={65}
                  outerRadius={105}
                  paddingAngle={10}
                  dataKey="value"
                  stroke="none"
                >
                  {order_distribution?.map((entry: any, index: number) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip 
                   contentStyle={{ backgroundColor: '#0f172a', border: '1px solid #ffffff10', borderRadius: '16px', boxShadow: '0 20px 50px rgba(0,0,0,0.5)' }}
                   itemStyle={{ fontSize: '11px', fontWeight: '900', textTransform: 'uppercase', fontFamily: 'monospace' }}
                />
                <Legend 
                  verticalAlign="bottom" 
                  align="center"
                  iconType="circle"
                  iconSize={8}
                  formatter={(value) => <span className="text-[10px] uppercase font-mono text-on-surface-variant font-black tracking-[0.1em] ml-2">{value}</span>}
                />
              </PieChart>
            </ResponsiveContainer>
            
            {/* Center Label */}
            <div className="absolute top-[45%] left-1/2 -translate-x-1/2 -translate-y-1/2 text-center pointer-events-none">
                <p className="text-[9px] font-mono text-on-surface-variant uppercase tracking-[0.3em] font-black opacity-50 mb-1">Total</p>
                <p className="text-3xl font-black text-white italic tracking-tighter leading-none">
                  {order_distribution?.reduce((acc: number, curr: any) => acc + curr.value, 0) || 0}
                </p>
            </div>
          </div>
        </GlassCard>

        {/* Daily Throughput */}
        <GlassCard className="lg:col-span-3 p-8 space-y-10">
           <div className="flex items-center justify-between border-b border-white/5 pb-5">
              <div className="flex items-center gap-4">
                <div className="p-2 rounded-lg bg-tertiary/5">
                  <TrendingUp className="w-4 h-4 text-tertiary" />
                </div>
                <h3 className="text-xs font-black text-white uppercase tracking-[0.2em]">Daily Throughput (7 Days)</h3>
              </div>
              <button className="text-[10px] font-black text-on-surface-variant hover:text-white transition-all uppercase tracking-widest bg-white/5 px-4 py-2 rounded-xl border border-white/5">
                 Extended View
              </button>
           </div>
           
           <div className="h-[240px] w-full">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={revenue_history?.slice(-7) || []}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#ffffff05" vertical={false} />
                  <XAxis 
                    dataKey="day" 
                    stroke="#94a3b830" 
                    fontSize={10} 
                    tickLine={false} 
                    axisLine={false} 
                    dy={10}
                    fontFamily="monospace"
                  />
                  <YAxis 
                    stroke="#94a3b830" 
                    fontSize={10} 
                    tickLine={false} 
                    axisLine={false}
                    tickFormatter={(val) => formatSmart(val).split(' ')[0]}
                    fontFamily="monospace"
                  />
                  <Tooltip content={<CustomTooltip />} cursor={{ fill: '#ffffff05' }} />
                  <Bar dataKey="revenue" fill="#B026FF" radius={[6, 6, 0, 0]} barSize={48} />
                </BarChart>
              </ResponsiveContainer>
           </div>
        </GlassCard>
      </div>
    </div>
  );
};

/* Sub-components & Helpers */

const SummaryCard: React.FC<{ 
  label: string; 
  value: string | number; 
  icon: any; 
  color: string;
  description?: string;
  trend?: string;
}> = ({ label, value, icon: Icon, color, description, trend }) => (
  <GlassCard className="p-7 relative overflow-hidden group hover:bg-white/[0.04] transition-all duration-700 hover:border-white/20">
    <div className={cn("absolute -top-10 -right-10 w-32 h-32 blur-[60px] opacity-10 pointer-events-none group-hover:opacity-30 transition-opacity duration-1000", color.replace('text-', 'bg-'))} />
    
    <div className="flex items-start justify-between relative z-10">
      <div className="space-y-6">
        <div className="flex items-center gap-3">
          <div className={cn("p-2.5 rounded-xl bg-black/40 border border-white/5 transition-transform duration-500 group-hover:scale-110", color)}>
            <Icon className="w-5 h-5" />
          </div>
          <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.3em] font-black">{label}</p>
        </div>
        
        <div className="space-y-2">
          <p className="text-3xl md:text-4xl font-black text-white italic tracking-tighter leading-none">{value}</p>
          {description && <p className="text-[10px] font-mono text-on-surface-variant/50 leading-none font-bold italic tracking-tight">{description}</p>}
        </div>
        
        {trend && (
          <div className="flex items-center gap-2 pt-3 border-t border-white/5">
            <CornerUpRight className="w-3 h-3 text-secondary" />
            <p className="text-[10px] font-mono text-secondary uppercase font-black tracking-widest italic py-0.5 px-2 bg-secondary/5 rounded-lg border border-secondary/10">
              {trend}
            </p>
          </div>
        )}
      </div>
    </div>
  </GlassCard>
);

const CustomTooltip = ({ active, payload, label }: any) => {
  if (active && payload && payload.length) {
    return (
      <div className="bg-[#0f172a]/95 border border-white/10 p-5 rounded-2xl shadow-[0_25px_50px_rgba(0,0,0,0.5)] backdrop-blur-2xl ring-1 ring-white/10">
        <p className="text-[10px] font-mono font-black text-primary uppercase tracking-[0.3em] mb-4 border-b border-white/10 pb-2">
          {label} Snapshot
        </p>
        <div className="space-y-2.5">
          {payload.map((item: any, i: number) => (
            <div key={i} className="flex items-center justify-between gap-10">
              <div className="flex items-center gap-2">
                 <div className="w-2 h-2 rounded-full" style={{ backgroundColor: item.stroke || item.fill }} />
                 <span className="text-[10px] font-mono text-white/50 uppercase font-black tracking-widest">{item.name}</span>
              </div>
              <span className="text-xs font-mono font-black italic tracking-tighter" style={{ color: item.stroke || item.fill }}>
                {typeof item.value === 'number' && item.name === 'revenue' ? formatSmart(item.value) : item.value}
              </span>
            </div>
          ))}
        </div>
      </div>
    );
  }
  return null;
};
