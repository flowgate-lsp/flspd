import React, { useState, useEffect, useMemo } from 'react';
import { GlassCard } from '../components/GlassCard';
import { LiquidityRadar } from '../components/LiquidityRadar';
import { BarChart3, TrendingUp, Loader2 } from 'lucide-react';
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer, BarChart, Bar, Cell } from 'recharts';
import { useAuth } from '../hooks/useAuth';
import { DashboardAPI } from '../lib/api';
import { formatSats } from '../lib/utils';


// Mock historical data — will be replaced with real API data
const revenueData = Array.from({ length: 14 }, (_, i) => ({
  day: `${14 - i}d`,
  revenue: Math.floor(Math.random() * 500000) + 100000,
})).reverse();

const orderVolume = [
  { name: 'CREATED', value: 5, color: '#807e80' },
  { name: 'PAID', value: 12, color: '#00baff' },
  { name: 'COMPLETED', value: 34, color: '#00fd87' },
  { name: 'FAILED', value: 3, color: '#ff3b30' },
];

export const Analytics: React.FC = () => {
  const { getAuthHeader } = useAuth();
  const [radarData, setRadarData] = useState<any[] | null>(null);
  const [loading, setLoading] = useState(true);

  const api = useMemo(() => {
    return new DashboardAPI(window.location.origin, getAuthHeader);
  }, [getAuthHeader]);

  useEffect(() => {
    api.getRadarData()
      .then(setRadarData)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [api]);

  if (loading) {
    return (
      <div className="h-[60vh] flex items-center justify-center">
        <Loader2 className="w-8 h-8 text-primary animate-spin" />
      </div>
    );
  }

  return (
    <div className="p-4 md:p-10 max-w-7xl mx-auto space-y-8 animate-in fade-in duration-700">
      <header>
        <h1 className="text-3xl md:text-4xl font-bold tracking-tighter text-white mb-2 uppercase italic leading-none">
          Liquidity Radar
        </h1>
        <p className="text-on-surface-variant font-mono text-xs tracking-widest uppercase">
          Performance Analytics & Revenue Tracking
        </p>
      </header>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Radar Chart */}
        <GlassCard delay={0.1} className="min-h-[400px]">
          <div className="flex items-center gap-3 mb-6">
            <div className="p-2 rounded-lg bg-primary/10">
              <TrendingUp className="w-5 h-5 text-primary" />
            </div>
            <h2 className="text-sm font-bold text-white uppercase tracking-widest">Node Performance</h2>
          </div>
          <LiquidityRadar data={radarData} />
        </GlassCard>

        {/* Revenue Area Chart */}
        <GlassCard delay={0.2} className="min-h-[400px]">
          <div className="flex items-center gap-3 mb-6">
            <div className="p-2 rounded-lg bg-secondary/10">
              <BarChart3 className="w-5 h-5 text-secondary" />
            </div>
            <h2 className="text-sm font-bold text-white uppercase tracking-widest">14-Day Revenue</h2>
          </div>
          <div className="h-[300px]">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={revenueData}>
                <defs>
                  <linearGradient id="revenueGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#00baff" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#00baff" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <XAxis
                  dataKey="day"
                  tick={{ fill: '#807e80', fontSize: 10 }}
                  axisLine={{ stroke: 'rgba(255,255,255,0.05)' }}
                  tickLine={false}
                />
                <YAxis
                  tick={{ fill: '#807e80', fontSize: 10 }}
                  axisLine={false}
                  tickLine={false}
                  tickFormatter={(v) => `${(v / 1000).toFixed(0)}k`}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#1c1c1f',
                    border: '1px solid rgba(255,255,255,0.1)',
                    borderRadius: '12px',
                    fontSize: '11px',
                    fontFamily: 'monospace',
                  }}
                  formatter={(value: any) => [formatSats(Number(value) || 0), 'Revenue']}
                  labelStyle={{ color: '#807e80' }}
                />
                <Area
                  type="monotone"
                  dataKey="revenue"
                  stroke="#00baff"
                  strokeWidth={2}
                  fill="url(#revenueGradient)"
                  animationDuration={1500}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </GlassCard>
      </div>

      {/* Order Volume Bar Chart */}
      <GlassCard delay={0.3}>
        <h2 className="text-sm font-bold text-white uppercase tracking-widest mb-6">Order Distribution by State</h2>
        <div className="h-[200px]">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={orderVolume} layout="vertical">
              <XAxis type="number" tick={{ fill: '#807e80', fontSize: 10 }} axisLine={false} tickLine={false} />
              <YAxis
                type="category"
                dataKey="name"
                tick={{ fill: '#adaaad', fontSize: 11, fontWeight: 'bold' }}
                axisLine={false}
                tickLine={false}
                width={90}
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: '#1c1c1f',
                  border: '1px solid rgba(255,255,255,0.1)',
                  borderRadius: '12px',
                  fontSize: '11px',
                  fontFamily: 'monospace',
                }}
              />
              <Bar dataKey="value" radius={[0, 8, 8, 0]} animationDuration={1200}>
                {orderVolume.map((entry, index) => (
                  <Cell key={`cell-${index}`} fill={entry.color} fillOpacity={0.7} />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </div>
      </GlassCard>
    </div>
  );
};
