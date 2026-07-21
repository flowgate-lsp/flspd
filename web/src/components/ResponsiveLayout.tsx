import React from 'react';
import { Sidebar } from './Sidebar';
import { BottomTabNav } from './BottomTabNav';
import { useMediaQuery } from '../hooks/useMediaQuery';
import { Activity } from 'lucide-react';
import { cn } from '../lib/utils';
import { useEvents } from '../hooks/useEvents';

interface ResponsiveLayoutProps {
  children: React.ReactNode;
}

export const ResponsiveLayout: React.FC<ResponsiveLayoutProps> = ({ 
  children
}) => {
  const isDesktop = useMediaQuery('(min-width: 1024px)');
  const { connected } = useEvents();

  return (
    <div className="min-h-screen bg-background transition-all duration-500 font-sans selection:bg-primary/30">
      {/* Background kinetic effect */}
      <div className="fixed inset-0 overflow-hidden pointer-events-none">
        <div className="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary/5 blur-[120px] rounded-full animate-pulse" />
        <div className="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-secondary/5 blur-[120px] rounded-full animate-pulse delay-700" />
      </div>

      {isDesktop ? (
        <>
          <Sidebar />
          <main className="pl-64 min-h-screen relative overflow-y-auto">
            {children}
          </main>
        </>
      ) : (
        <>
          {/* Mobile Header */}
          <header className="h-16 fixed top-0 left-0 right-0 z-40 bg-surface-container-lowest/90 backdrop-blur-xl border-b border-outline-variant flex items-center justify-between px-6">
            <div className="flex items-center gap-2">
              <div className="w-6 h-6 rounded bg-primary/20 flex items-center justify-center">
                <Activity className="w-3.5 h-3.5 text-primary" />
              </div>
              <span className="font-mono font-bold text-sm text-white uppercase tracking-tighter">flspd</span>
            </div>
            {/* Context Badge */}
            <div className={cn(
              "px-3 py-1 rounded-full border transition-colors",
              connected ? "bg-secondary/10 border-secondary/20" : "bg-error/10 border-error/20"
            )}>
               <span className={cn(
                 "text-[10px] font-mono font-bold uppercase",
                 connected ? "text-secondary animate-pulse" : "text-error"
               )}>
                 {connected ? "Node Live" : "Disconnected"}
               </span>
            </div>
          </header>
          
          <main className="pt-16 pb-24 min-h-screen px-4">
            {children}
          </main>
          
          <BottomTabNav />
        </>
      )}
    </div>
  );
};
