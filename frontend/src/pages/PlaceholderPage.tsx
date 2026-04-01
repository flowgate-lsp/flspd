import React from 'react';
import { GlassCard } from '../components/GlassCard';

export const PlaceholderPage: React.FC<{ title: string }> = ({ title }) => {
  return (
    <div className="p-6 h-full flex flex-col items-center justify-center min-h-[60vh] animate-in fade-in slide-in-from-bottom-4 duration-1000">
      <GlassCard className="p-12 text-center max-w-md">
        <h2 className="text-2xl font-bold text-white mb-4 uppercase tracking-widest">{title}</h2>
        <div className="w-16 h-1 w-full bg-primary/20 rounded-full mb-6 relative overflow-hidden">
           <div className="absolute inset-0 bg-primary animate-shimmer" />
        </div>
        <p className="text-on-surface-variant text-sm font-mono leading-relaxed">
          The <span className="text-secondary">[{title.toLowerCase()}]</span> module is currently integrating with the flspd core. 
          Deployment status: <span className="animate-pulse text-primary font-bold">READY_SOON</span>
        </p>
      </GlassCard>
    </div>
  );
};
