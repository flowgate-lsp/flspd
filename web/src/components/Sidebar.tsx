import React from 'react';
import { List, Zap, Settings, Activity, Gauge, BarChart3, Wifi, WifiOff } from 'lucide-react';
import { Link } from '@tanstack/react-router';
import { cn } from '../lib/utils';
import { useEvents } from '../hooks/useEvents';
import { useSystemInfo } from '../hooks/useSystemInfo';

const navItems = [
  { id: 'overview', label: 'Command Center', icon: Gauge },
  { id: 'orders', label: 'Order Matrix', icon: List },
  { id: 'nodes', label: 'Peer Nodes', icon: Activity },
  { id: 'jit', label: 'JIT Channels', icon: Zap },
  { id: 'analytics', label: 'Liquidity Radar', icon: BarChart3 },
  { id: 'settings', label: 'Settings', icon: Settings },
];

export const Sidebar: React.FC = () => {
  const { connected } = useEvents();
  const { info } = useSystemInfo();
  return (
    <aside className="w-64 h-screen fixed left-0 top-0 bg-surface-container-lowest border-r border-outline-variant flex flex-col p-6 z-50">
      <div className="flex items-center gap-3 mb-10 px-2 text-primary">
        <div className="w-8 h-8 rounded-lg bg-primary/20 flex items-center justify-center animate-glow-pulse">
          <Activity className="w-5 h-5 text-primary" />
        </div>
        <span className="font-mono text-xl font-bold tracking-tight text-white uppercase">flspd</span>
      </div>

      <nav className="flex-1 space-y-2">
        {navItems.map((item) => {
          const to = item.id === 'overview' ? '/' : `/${item.id}`
          return (
            <Link
              key={item.id}
              to={to}
            >
              {({ isActive }: { isActive: boolean }) => (
                <div className={cn(
                  "w-full flex items-center gap-3 px-3 py-3 rounded-xl transition-all duration-300 group hover:bg-white/5 cursor-pointer",
                  isActive ? "bg-primary/10 text-white" : "text-on-surface-variant hover:text-white"
                )}>
                  <item.icon className={cn(
                    "w-5 h-5 transition-colors",
                    isActive ? "text-primary" : "text-inherit group-hover:text-primary"
                  )} />
                  <span className="text-sm font-medium">{item.label}</span>
                </div>
              )}
            </Link>
          )
        })}
      </nav>

      <div className="mt-auto pt-6">
        <div className="rounded-xl border border-outline-variant bg-white/[0.03] p-3.5 space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className={cn(
                "w-1.5 h-1.5 rounded-full",
                connected ? "bg-secondary shadow-[0_0_8px_rgba(0,253,135,0.6)] animate-pulse" : "bg-tertiary shadow-[0_0_8px_rgba(255,186,0,0.6)] animate-pulse"
              )} />
              <span className={cn(
                "text-[10px] font-mono uppercase font-black tracking-widest",
                connected ? "text-secondary" : "text-tertiary"
              )}>
                {connected ? "Live" : "Reconnecting…"}
              </span>
            </div>
            {connected ? (
              <Wifi className="w-3.5 h-3.5 text-secondary/60" />
            ) : (
              <WifiOff className="w-3.5 h-3.5 text-tertiary/60" />
            )}
          </div>

          {info && info.nodes.length > 0 && (
            <div className="pt-2.5 border-t border-outline-variant/60 space-y-2">
              {info.nodes.map((n) => (
                <div key={n.name} className="flex items-center justify-between gap-2">
                  <span className="text-[9px] font-mono uppercase tracking-widest text-white/50 truncate">
                    {n.name}
                  </span>
                  <span className={cn(
                    "shrink-0 text-[8px] font-mono font-black uppercase tracking-widest px-1.5 py-0.5 rounded",
                    n.connected ? "text-secondary bg-secondary/10" : "text-tertiary bg-tertiary/10"
                  )}>
                    {n.connected ? "Up" : "Down"}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </aside>
  );
};
