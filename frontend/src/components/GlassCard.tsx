import { motion, type HTMLMotionProps } from 'framer-motion';
import { cn } from '../lib/utils';

interface GlassCardProps extends HTMLMotionProps<"div"> {
  children: React.ReactNode;
  delay?: number;
  glow?: boolean;
}

export const GlassCard: React.FC<GlassCardProps> = ({ 
  children, 
  className, 
  delay = 0, 
  glow = false,
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
      className={cn(
        "glass-panel p-6 relative overflow-hidden group",
        glow && "after:absolute after:inset-0 after:bg-primary/5 after:opacity-0 group-hover:after:opacity-100 after:transition-opacity after:duration-500",
        className
      )}
      {...props}
    >
      {/* Subtle Gradient Glow */}
      <div className="absolute -top-24 -right-24 w-48 h-48 bg-primary/10 blur-[80px] rounded-full pointer-events-none group-hover:bg-primary/20 transition-colors duration-500" />
      
      <div className="relative z-10">
        {children}
      </div>
    </motion.div>
  );
};
