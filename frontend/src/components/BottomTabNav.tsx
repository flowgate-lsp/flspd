import React from 'react';
import { List, Zap, Settings, Activity, Gauge, BarChart3 } from 'lucide-react';
import { cn } from '../lib/utils';

const navItems = [
  { id: 'overview', label: 'Home', icon: Gauge },
  { id: 'orders', label: 'Orders', icon: List },
  { id: 'nodes', label: 'Nodes', icon: Activity },
  { id: 'jit', label: 'JIT', icon: Zap },
  { id: 'analytics', label: 'Radar', icon: BarChart3 },
  { id: 'settings', label: 'Props', icon: Settings },
];

interface BottomTabNavProps {
  activePage: string;
  onPageChange: (id: string) => void;
}

export const BottomTabNav: React.FC<BottomTabNavProps> = ({ activePage, onPageChange }) => {
  return (
    <nav className="fixed bottom-0 left-0 right-0 h-20 bg-surface-container-lowest/80 backdrop-blur-xl border-t border-white/5 flex items-center justify-around px-2 z-50 pb-safe">
      {navItems.map((item) => (
        <button
          key={item.id}
          onClick={() => onPageChange(item.id)}
          className={cn(
            "flex flex-col items-center gap-1.5 px-3 py-2 rounded-2xl transition-all duration-300",
            activePage === item.id ? "text-primary" : "text-on-surface-variant hover:text-white"
          )}
        >
          <item.icon className={cn(
            "w-6 h-6 transition-transform duration-300",
            activePage === item.id ? "scale-110" : "scale-100"
          )} />
          <span className="text-[10px] font-medium uppercase tracking-wider">{item.label}</span>
          
          {activePage === item.id && (
            <div className="absolute -bottom-1 w-1 h-1 rounded-full bg-primary shadow-[0_0_8px_rgba(0,186,255,0.8)]" />
          )}
        </button>
      ))}
    </nav>
  );
};
