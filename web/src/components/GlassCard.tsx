import { motion, type HTMLMotionProps } from 'framer-motion';
import { cn } from '../lib/utils';

type GlassCardAccent = 'none' | 'primary' | 'secondary' | 'tertiary' | 'error';

interface GlassCardProps extends HTMLMotionProps<"div"> {
  children: React.ReactNode;
  delay?: number;
  /** Corner glow color. Only set this when the card's content actually
   * means something in that color (e.g. an order needing attention gets
   * `tertiary`) -- it's a status signal, not decoration, so it defaults to
   * off rather than a permanent primary-tinted blob on every card. */
  accent?: GlassCardAccent;
}

const ACCENT_GLOW: Record<Exclude<GlassCardAccent, 'none'>, string> = {
  primary: 'bg-primary/10 group-hover/card:bg-primary/20',
  secondary: 'bg-secondary/10 group-hover/card:bg-secondary/20',
  tertiary: 'bg-tertiary/10 group-hover/card:bg-tertiary/20',
  error: 'bg-error/10 group-hover/card:bg-error/20',
};

export const GlassCard: React.FC<GlassCardProps> = ({
  children,
  className,
  delay = 0,
  accent = 'none',
  ...props
}) => {
  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{
        duration: 0.8,
        delay: delay,
        ease: [0.16, 1, 0.3, 1]
      }}
      className={cn("glass-panel p-6 relative overflow-hidden group/card", className)}
      {...props}
    >
      {accent !== 'none' && (
        <div className={cn(
          "absolute -top-24 -right-24 w-48 h-48 blur-[80px] rounded-full pointer-events-none transition-colors duration-500",
          ACCENT_GLOW[accent]
        )} />
      )}

      <div className="relative z-10 flex flex-col flex-1 min-h-0">
        {children}
      </div>
    </motion.div>
  );
};
