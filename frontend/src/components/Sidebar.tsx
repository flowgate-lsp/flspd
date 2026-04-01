import React from 'react';
import { List, Zap, Settings, Activity, Gauge, BarChart3 } from 'lucide-react';
import { cn } from '../lib/utils';
import { useEvents } from '../hooks/useEvents';

const navItems = [
  { id: 'overview', label: 'Command Center', icon: Gauge },
  { id: 'orders', label: 'Order Matrix', icon: List },
  { id: 'nodes', label: 'Peer Nodes', icon: Activity },
  { id: 'jit', label: 'JIT Channels', icon: Zap },
  { id: 'analytics', label: 'Liquidity Radar', icon: BarChart3 },
  { id: 'settings', label: 'Settings', icon: Settings },
];

interface SidebarProps {
  activePage: string;
  onPageChange: (page: string) => void;
}

export const Sidebar: React.FC<SidebarProps> = ({ activePage, onPageChange }) => {
  const { connected } = useEvents();
  return (
    <aside className="w-64 h-screen fixed left-0 top-0 bg-surface-container-lowest border-r border-white/5 flex flex-col p-6 z-50">
      <div className="flex items-center gap-3 mb-10 px-2 text-primary">
        <div className="w-8 h-8 rounded-lg bg-primary/20 flex items-center justify-center animate-glow-pulse">
          <Activity className="w-5 h-5 text-primary" />
        </div>
        <span className="font-mono text-xl font-bold tracking-tight text-white uppercase">flspd</span>
      </div>

      <nav className="flex-1 space-y-2">
        {navItems.map((item) => (
          <button
            key={item.id}
            onClick={() => onPageChange(item.id)}
            className={cn(
              "w-full flex items-center gap-3 px-3 py-3 rounded-xl transition-all duration-300 group hover:bg-white/5",
              activePage === item.id ? "bg-primary/10 text-white" : "text-on-surface-variant hover:text-white"
            )}
          >
            <item.icon className={cn(
              "w-5 h-5 transition-colors",
              activePage === item.id ? "text-primary" : "text-inherit group-hover:text-primary"
            )} />
            <span className="text-sm font-medium">{item.label}</span>
          </button>
        ))}
      </nav>

      <div className="mt-auto pt-6 border-t border-white/5">
        <div className="flex items-center gap-3 px-2">
          <div className={cn(
            "w-2 h-2 rounded-full",
            connected ? "bg-secondary shadow-[0_0_8px_rgba(0,253,135,0.5)]" : "bg-error shadow-[0_0_8px_rgba(255,82,82,0.5)]"
          )} />
          <span className={cn(
            "text-xs font-mono uppercase font-bold tracking-widest",
            connected ? "text-secondary" : "text-error"
          )}>
            {connected ? "Node Online" : "Connection Error"}
          </span>
        </div>
      </div>
    </aside>
  );
};
