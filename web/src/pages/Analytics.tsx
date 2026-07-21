import React, { useState, useEffect, useRef } from 'react';
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
  Cell
} from 'recharts';
import { useFormatAmount, cn } from '../lib/utils';
import { useDashboard } from '../hooks/useDashboard';
import { TrendingUp, Users, Zap, Award, Loader2, BarChart3, PieChart as PieIcon, Activity } from 'lucide-react';

export const Analytics: React.FC = () => {
  const { api } = useDashboard();
  const { format } = useFormatAmount();
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const legendRef = useRef<HTMLDivElement>(null);
  const [legendOverflowing, setLegendOverflowing] = useState(false);

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

  const totalOrders = data?.order_distribution?.reduce((acc: number, curr: any) => acc + curr.value, 0) || 0;
  const filteredDistribution = data?.order_distribution?.filter((d: any) => d.value > 0) || [];

  // Drives the bottom scroll-fade hint on the breakdown list -- only shown
  // once there's actually more to scroll to, not as permanent decoration.
  useEffect(() => {
    const el = legendRef.current;
    if (!el) {
      setLegendOverflowing(false);
      return;
    }
    const evaluate = () => setLegendOverflowing(el.scrollHeight - el.scrollTop - el.clientHeight > 4);
    evaluate();
    el.addEventListener('scroll', evaluate);
    window.addEventListener('resize', evaluate);
    return () => {
      el.removeEventListener('scroll', evaluate);
      window.removeEventListener('resize', evaluate);
    };
  }, [filteredDistribution.length]);

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
    <div className="p-3 md:p-10 max-w-7xl mx-auto space-y-12 animate-in fade-in slide-in-from-bottom-2 duration-500">
      {/* Header Section */}
      <header className="flex flex-col md:flex-row md:items-end justify-between gap-8 pb-6">
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
          <div className="flex items-center gap-2 bg-white/5 px-3 py-1.5 rounded-lg border border-outline-variant">
            <span className="w-2 h-2 rounded-full bg-secondary shadow-[0_0_10px_rgba(0,253,135,0.6)] animate-pulse" />
            Live Sync
          </div>
          <span className="hidden md:inline text-white/10">|</span>
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
          value={format(total_revenue || 0)} 
          icon={TrendingUp} 
          color="text-secondary" 
          description="Inbound fees earned"
          trend="+12.4% vs 7d"
          showTrend={revenue_history?.length > 1}
        />
        <SummaryCard 
          label="Success Rate" 
          value={`${(success_rate || 0).toFixed(1)}%`} 
          icon={Award} 
          color="text-primary" 
          description="Channel funding success"
          trend="Optimal Flow"
          showTrend={success_rate > 90}
        />
        <SummaryCard 
          label="Avg Order Size" 
          value={format(avg_order_size || 0)} 
          icon={Zap} 
          color="text-tertiary" 
          description="Mean liquidity request"
        />
        <SummaryCard 
          label="Connected Peers" 
          value={active_peers || 0} 
          icon={Users} 
          color="text-white" 
          description="Active node connections"
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-10 items-start">
        {/* Main Revenue Chart */}
        <GlassCard className="lg:col-span-2 p-5 md:p-8 flex flex-col h-[350px] md:h-[520px] bg-surface-container-high/40 border-outline-variant">
          <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between mb-8 sm:mb-10 pb-5 gap-4">
            <div className="flex items-center gap-4">
              <div className="p-2 rounded-lg bg-primary/5">
                <BarChart3 className="w-4 h-4 text-primary" />
              </div>
              <h3 className="text-xs font-black text-white uppercase tracking-[0.2em]">Revenue & Order Velocity</h3>
            </div>
            <div className="flex gap-6">
               <div className="flex items-center gap-2 text-[10px] font-mono text-on-surface-variant font-bold uppercase tracking-widest">
                   <div className="w-2.5 h-2.5 rounded-full bg-primary shadow-[0_0_12px_rgba(0,186,255,0.6)]" /> Revenue (Fees)
               </div>
               <div className="flex items-center gap-2 text-[10px] font-mono text-on-surface-variant font-bold uppercase tracking-widest">
                  <div className="w-2.5 h-2.5 rounded-full bg-secondary shadow-[0_0_12px_rgba(0,253,135,0.6)]" /> Capacity (Volume)
               </div>
            </div>
          </div>
          
          <div className="mt-auto w-full h-[220px] md:h-[400px] relative px-0 md:px-2">
            {(!revenue_history || revenue_history.length === 0) ? (
              <NoData label="No revenue history recorded yet" />
            ) : (
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
                <CartesianGrid strokeDasharray="3 3" stroke="#ffffff0f" vertical={false} />
                <XAxis 
                  dataKey="date" 
                  stroke="#94a3b820" 
                  fontSize={9} 
                  tickLine={false} 
                  axisLine={false}
                  dy={15}
                  fontFamily="monospace"
                  tickFormatter={(val) => {
                    // val is "2026-04-01"
                    if (!val || typeof val !== 'string') return '';
                    const parts = val.split('-');
                    if (parts.length < 3) return val;
                    // Just return day/month or weekday if we can parse it simply
                    const d = new Date(val);
                    const days = ['SUN', 'MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT'];
                    return isNaN(d.getTime()) ? parts[2] : days[d.getDay()];
                  }}
                />
                <YAxis 
                  stroke="#94a3b830" 
                  fontSize={10} 
                  tickLine={false} 
                  axisLine={false}
                  tickFormatter={(val) => format(val).split(' ')[0]}
                  fontFamily="monospace"
                />
                <Tooltip 
                  content={<CustomTooltip format={format} />}
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
                  dataKey="volume" 
                  stroke="#00FD87" 
                  strokeWidth={2} 
                  strokeDasharray="4 4"
                  fillOpacity={1} 
                  fill="url(#colorVolume)" 
                  animationDuration={2000}
                />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </div>
        </GlassCard>

        {/* Order Composition (Pie) */}
        <GlassCard className="lg:col-span-1 p-5 md:p-8 flex flex-col h-auto md:h-[520px] bg-surface-container-high/40 border-outline-variant">
          <div className="flex items-center gap-4 mb-8 md:mb-10 pb-5">
            <div className="p-2 rounded-lg bg-secondary/5">
              <PieIcon className="w-4 h-4 text-secondary" />
            </div>
            <h3 className="text-xs font-black text-white uppercase tracking-[0.2em]">Order Distribution</h3>
          </div>
          
          <div className="flex-1 w-full min-h-0 relative flex flex-col items-center">
            {(!order_distribution || order_distribution.length === 0) ? (
              <NoData label="No orders distributed in this sector" />
            ) : (
              <>
                <div className="w-full h-[200px] md:h-[240px] shrink-0 relative">
                  {/* Center Label */}
                  <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 text-center pointer-events-none z-0">
                      <p className="text-[8px] font-mono text-on-surface-variant uppercase tracking-[0.4em] font-black opacity-40 mb-1">Total Signals</p>
                      <p className="text-4xl font-black text-white italic tracking-tighter leading-none shadow-sm tabular-nums">
                        {totalOrders}
                      </p>
                  </div>

                  <ResponsiveContainer width="100%" height="100%" className="relative z-10">
                    <PieChart>
                      <Pie
                        data={filteredDistribution.length > 0 ? filteredDistribution : [{ name: 'EMPTY', value: 1, color: '#ffffff05' }]}
                        cx="50%"
                        cy="50%"
                        innerRadius={60}
                        outerRadius={88}
                        paddingAngle={filteredDistribution.length > 1 ? 6 : 0}
                        dataKey="value"
                        nameKey="name"
                        stroke="none"
                        animationDuration={1500}
                      >
                        {(filteredDistribution.length > 0 ? filteredDistribution : [{ name: 'EMPTY', value: 1, color: '#ffffff05' }]).map((entry: any, index: number) => (
                           <Cell
                             key={`cell-${index}`}
                             fill={entry.color}
                             stroke={entry.color}
                             strokeOpacity={0.2}
                             strokeWidth={1}
                           />
                        ))}
                      </Pie>
                      <Tooltip
                        content={<CustomTooltip format={format} />}
                        cursor={{ stroke: '#ffffff10', strokeWidth: 1 }}
                      />
                    </PieChart>
                  </ResponsiveContainer>
                </div>

                {/* Breakdown List -- bounded to the remaining card height so it
                    actually scrolls in place instead of silently overflowing
                    past the card edge. Fade hints only render once there's
                    real overflow to scroll to. */}
                <div className="relative w-full flex-1 min-h-0 mt-6">
                  <div ref={legendRef} className="h-full w-full overflow-y-auto pr-2 custom-scrollbar space-y-2.5">
                    {filteredDistribution?.map((item: any) => {
                      const percentage = totalOrders > 0 ? Math.round((item.value / totalOrders) * 100) : 0;
                      return (
                          <div key={item.name} className="flex items-center justify-between group py-1.5 border-b border-outline-variant last:border-0 hover:bg-white/[0.03] px-2 rounded-lg transition-colors">
                            <div className="flex items-center gap-3">
                              <div className="w-2 h-2 rounded-full shadow-[0_0_8px_rgba(255,255,255,0.1)]" style={{ backgroundColor: item.color }} />
                              <span className="text-[10px] font-mono text-on-surface-variant group-hover:text-white transition-colors uppercase font-black tracking-widest">{item.name}</span>
                            </div>
                            <div className="flex items-center gap-4">
                              <span className="text-[10px] font-mono text-white/40 font-bold italic">{percentage}%</span>
                              <span className="text-[11px] font-mono text-white font-black italic min-w-[30px] text-right tabular-nums">{item.value}</span>
                            </div>
                          </div>
                      );
                    })}
                  </div>
                  {legendOverflowing && (
                    <div className="pointer-events-none absolute bottom-0 left-0 right-0 h-9 bg-gradient-to-t from-surface-container-high/90 via-surface-container-high/50 to-transparent rounded-b-xl" />
                  )}
                </div>
              </>
            )}
          </div>
        </GlassCard>

        {/* Daily Throughput */}
        <GlassCard className="lg:col-span-3 p-8 space-y-10 bg-surface-container-high/40 border-outline-variant">
            <div className="flex items-center justify-between pb-5">
               <div className="flex items-center gap-4">
                 <div className="p-2 rounded-lg bg-tertiary/5">
                   <TrendingUp className="w-4 h-4 text-tertiary" />
                 </div>
                 <h3 className="text-xs font-black text-white uppercase tracking-[0.2em]">Daily Throughput (7 Days)</h3>
               </div>
            </div>
           
           <div className="h-[240px] w-full relative">
              {(!revenue_history || revenue_history.length === 0) ? (
                <NoData label="No throughput telemetry identified" />
              ) : (
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={revenue_history?.slice(-7) || []}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#ffffff0f" vertical={false} />
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
                    tickFormatter={(val) => format(val).split(' ')[0]}
                    fontFamily="monospace"
                  />
                  <Tooltip content={<CustomTooltip format={format} />} cursor={{ fill: '#ffffff05' }} />
                  {/* Same metric as the Revenue line above -- same color (primary),
                      not the unrelated purple this used to be, so "revenue" reads
                      as one consistent color across both charts on this page. */}
                  <Bar dataKey="revenue" fill="#00BAFF" radius={[6, 6, 0, 0]} barSize={48} />
                  </BarChart>
                </ResponsiveContainer>
              )}
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
  showTrend?: boolean;
}> = ({ label, value, icon: Icon, color, description, trend, showTrend }) => {
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
            </p>
        </div>
        <Icon className={cn("w-4 h-4 opacity-40 group-hover:opacity-100 transition-opacity duration-500", color)} />
      </div>
      
      <div>
        <div className="flex items-baseline gap-2 mb-2">
          <p className="text-3xl md:text-4xl font-black text-white tracking-tighter tabular-nums leading-[0.8]">{num}</p>
          {suffix && <span className="text-[10px] font-mono text-white/30 uppercase font-black tracking-widest">{suffix}</span>}
        </div>
        {description && <p className="text-[9px] font-mono text-on-surface-variant/40 uppercase tracking-widest leading-none font-bold italic truncate block pt-1">{description}</p>}
        {trend && showTrend && (
          <div className="flex items-center gap-1.5 pt-2">
            <span className={cn(
              "text-[9px] font-black font-mono px-2 py-0.5 rounded border tracking-widest uppercase", 
              trend.startsWith('+') || trend === "Optimal Flow" ? "text-secondary border-secondary/20 bg-secondary/5" : "text-white/40 border-outline bg-white/5"
            )}>
              {trend}
            </span>
          </div>
        )}
      </div>
    </div>
  );
};

const NoData: React.FC<{ label: string }> = ({ label }) => (
  <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
    <div className="flex flex-col items-center gap-4 group opacity-40 hover:opacity-60 transition-opacity duration-1000">
      <BarChart3 className="w-8 h-8 text-on-surface-variant group-hover:scale-110 transition-transform" />
      <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.4em] font-black italic text-center px-10">
        {label}
      </p>
    </div>
  </div>
);

const CustomTooltip = ({ active, payload, label, format }: any) => {
  if (active && payload && payload.length) {
    return (
      <div className="relative z-50 bg-surface-container-high/95 border border-outline p-5 rounded-2xl shadow-[0_25px_50px_rgba(0,0,0,0.5)] backdrop-blur-2xl ring-1 ring-white/10">
        <p className="text-[10px] font-mono font-black text-primary uppercase tracking-[0.3em] mb-4 border-b border-outline pb-2">
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
                {typeof item.value === 'number' && (item.name === 'revenue' || item.name === 'volume') ? format(item.value) : item.value}
              </span>
            </div>
          ))}
        </div>
      </div>
    );
  }
  return null;
};
