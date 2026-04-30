import React, { useState, useEffect } from 'react';
import { GlassCard } from '../components/GlassCard';
import { 
    Settings as SettingsIcon, Save, RefreshCw, Zap, Shield, Globe, 
    Loader2, AlertCircle, CheckCircle2, HelpCircle, ArrowRightLeft,
    Network, Clock, DollarSign, Percent, Scale, Activity
} from 'lucide-react';
import { useSettings } from '../hooks/useSettings';
import { useUnit, type DisplayUnit } from '../contexts/UnitContext';
import { formatAmount, formatInput, cn } from '../lib/utils';
import { motion, AnimatePresence } from 'framer-motion';
import { CurrencyConverterModal } from '../components/CurrencyConverterModal';
import { InfoModal } from '../components/InfoModal';

type Tab = 'general' | 'jit' | 'lsps1';

// Enhanced FinancialInput for modern look and long numbers
const FinancialInput: React.FC<{ 
    field: string;
    label: string; 
    explanation: string; 
    value: any; 
    source: 'LOKI' | 'MLOKI';
    onEdit: (field: string, source: 'LOKI' | 'MLOKI', value: number, label: string, explanation: string) => void;
    onInfo: (label: string, explanation: string) => void;
}> = ({ field, label, explanation, value, source, onEdit, onInfo }) => (
  <div className="space-y-3 group">
    <div className="flex justify-between items-center px-1">
      <div className="flex items-center gap-2">
        <label className="text-[10px] font-black text-on-surface-variant uppercase tracking-[0.2em] group-focus-within:text-primary transition-colors">
            {label}
        </label>
        <button type="button" onClick={() => onInfo(label, explanation)} className="text-on-surface-variant/40 hover:text-primary transition-colors">
            <HelpCircle className="w-4 h-4" />
        </button>
      </div>
      <button 
        type="button" 
        onClick={() => onEdit(field, source, value, label, explanation)} 
        className="text-[9px] text-primary hover:text-white transition-all font-mono uppercase tracking-[0.15em] font-black flex items-center gap-1.5 bg-primary/5 hover:bg-primary/20 px-2.5 py-1 rounded-lg border border-primary/20"
      >
        <ArrowRightLeft className="w-3 h-3" />
        Convert
      </button>
    </div>
    
    <div className="relative overflow-hidden group/input">
        <div className="w-full bg-surface-container-lowest/80 border border-white/5 rounded-2xl p-4 flex flex-col gap-2 group-focus-within/input:border-primary/40 group-focus-within/input:ring-4 group-focus-within/input:ring-primary/5 transition-all duration-500">
            <div className="flex items-baseline justify-between gap-4">
                <span className="text-xl md:text-2xl font-mono text-white tracking-tighter truncate selection:bg-primary/30">
                    {formatInput(value)}
                </span>
                <span className="text-[9px] font-black text-on-surface-variant uppercase tracking-widest bg-white/5 px-2 py-0.5 rounded-md border border-white/5 shrink-0">
                    {source}
                </span>
            </div>
            
            <div className="flex items-center gap-2.5 pt-2 border-t border-white/5">
                <div className="w-1.5 h-1.5 rounded-full bg-secondary shadow-[0_0_8px_rgba(0,253,135,0.4)]" />
                <span className="text-[11px] font-mono text-secondary font-bold tracking-tight">
                    {formatAmount(value, 'AUTO', source)}
                </span>
            </div>
        </div>
        {/* Subtle decorative glow */}
        <div className="absolute inset-0 bg-primary/2 opacity-0 group-hover/input:opacity-100 transition-opacity pointer-events-none" />
    </div>
  </div>
);

const Input: React.FC<{ 
    label: string; 
    explanation: string; 
    value: any; 
    onChange: (v: any) => void; 
    type?: string; 
    placeholder?: string; 
    step?: string; 
    icon?: any;
    onInfo: (label: string, explanation: string) => void 
}> = ({ 
  label, explanation, value, onChange, type = 'text', placeholder, step, icon: Icon, onInfo
}) => (
  <div className="space-y-3 group">
    <div className="flex items-center gap-2 px-1">
        <label className="text-[10px] font-black text-on-surface-variant uppercase tracking-[0.2em] group-focus-within:text-primary transition-colors">
            {label}
        </label>
        <button type="button" onClick={() => onInfo(label, explanation)} className="text-on-surface-variant/40 hover:text-primary transition-colors">
            <HelpCircle className="w-4 h-4" />
        </button>
    </div>
    <div className="relative">
        {Icon && <Icon className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-on-surface-variant/40 group-focus-within:text-primary transition-colors" />}
        <input 
          type={type}
          step={step}
          value={value ?? ''}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          className={cn(
            "w-full bg-surface-container-lowest/80 border border-white/5 rounded-2xl py-4 pr-4 text-sm text-white font-mono placeholder:text-white/10 focus:outline-none focus:border-primary/40 focus:ring-4 focus:ring-primary/5 transition-all duration-500",
            Icon ? "pl-11" : "pl-4"
          )}
        />
    </div>
  </div>
);

const SectionHeader: React.FC<{ title: string; subtitle: string; icon: any }> = ({ title, subtitle, icon: Icon }) => (
    <div className="flex items-center gap-4 mb-8">
        <div className="w-12 h-12 rounded-2xl bg-primary/10 border border-primary/20 flex items-center justify-center text-primary shadow-[0_8px_24px_rgba(0,186,255,0.15)]">
            <Icon className="w-6 h-6" />
        </div>
        <div>
            <h3 className="text-lg font-black text-white uppercase italic tracking-tighter leading-none">{title}</h3>
            <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.15em] mt-1">{subtitle}</p>
        </div>
    </div>
);

const Switch: React.FC<{ checked: boolean; onChange: (v: boolean) => void }> = ({ checked, onChange }) => (
  <button 
    type="button"
    onClick={() => onChange(!checked)}
    className={cn(
        "w-14 h-7 rounded-full transition-all duration-500 relative p-1",
        checked ? "bg-primary shadow-[0_0_20px_rgba(0,186,255,0.3)]" : "bg-white/5 border border-white/10"
    )}
  >
    <div className={cn(
        "w-5 h-5 rounded-full transition-all duration-500 flex items-center justify-center",
        checked ? "translate-x-7 bg-black" : "translate-x-0 bg-white/20"
    )}>
        {checked ? <Zap className="w-3 h-3 text-primary fill-current" /> : null}
    </div>
  </button>
);

export const Settings: React.FC = () => {
  const { config, loading, saving, error, refreshConfig, updateConfig } = useSettings();
  const { unit, setUnit } = useUnit();
  const [activeTab, setActiveTab] = useState<Tab>((localStorage.getItem('settings_tab') as Tab) || 'general');
  const [formData, setFormData] = useState<any>(null);
  const [showSuccess, setShowSuccess] = useState(false);
  const [editingField, setEditingField] = useState<{ field: string; source: 'LOKI' | 'MLOKI'; value: number; label: string; explanation: string } | null>(null);
  const [infoModal, setInfoModal] = useState<{ label: string; explanation: string } | null>(null);

  useEffect(() => {
    if (config) {
      setFormData({ ...config });
    }
  }, [config]);

  const handleInputChange = (path: string, value: any) => {
    setFormData((prev: any) => {
      const next = JSON.parse(JSON.stringify(prev));
      const keys = path.split('.');
      let current = next;
      for (let i = 0; i < keys.length - 1; i++) {
        current = current[keys[i]];
      }
      current[keys[keys.length - 1]] = value;
      return next;
    });
  };

  const handleSave = async () => {
    const success = await updateConfig(formData);
    if (success) {
      setShowSuccess(true);
      setTimeout(() => setShowSuccess(false), 3000);
    }
  };

  if (loading && !formData) {
    return (
      <div className="h-[60vh] flex flex-col items-center justify-center gap-4">
        <Loader2 className="w-10 h-10 text-primary animate-spin" />
        <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest animate-pulse">Initializing Node Config...</p>
      </div>
    );
  }

  const tabs: { id: Tab; label: string; icon: any }[] = [
    { id: 'general', label: 'Identity', icon: Globe },
    { id: 'jit', label: 'LSPS2 / JIT', icon: Zap },
    { id: 'lsps1', label: 'LSPS1 Protocol', icon: Shield },
  ];

  return (
    <div className="p-4 md:p-10 max-w-6xl mx-auto space-y-8 md:space-y-12 animate-in fade-in duration-1000 pb-40">
      {/* Header Section */}
      <div className="flex flex-col md:flex-row md:justify-between md:items-center gap-8 bg-surface-variant/20 p-6 md:p-8 rounded-[2rem] border border-white/5 backdrop-blur-md relative overflow-hidden group">
        <div className="absolute -right-20 -top-20 w-64 h-64 bg-primary/5 blur-[100px] rounded-full" />
        
        <div className="space-y-3 relative z-10">
          <div className="flex items-center gap-3">
            <div className="p-2.5 bg-primary/10 rounded-xl border border-primary/20">
                <SettingsIcon className="w-6 h-6 text-primary animate-[spin_10s_linear_infinite]" />
            </div>
            <h1 className="text-3xl md:text-5xl font-black tracking-tighter text-white uppercase italic leading-none">
                Settings
            </h1>
          </div>
          <p className="text-on-surface-variant font-mono text-[10px] md:text-xs tracking-[0.2em] uppercase pl-1">
             Manage <span className="text-white">LSP Protocols</span> & Infrastructure
          </p>
        </div>

        <div className="flex gap-4 w-full md:w-auto relative z-10">
          <button 
            onClick={refreshConfig}
            className="group/refresh p-4 rounded-2xl bg-white/5 border border-white/10 text-white hover:bg-white/10 transition-all active:scale-95 flex items-center justify-center"
            title="Reload configuration"
          >
            <RefreshCw className={cn("w-5 h-5 transition-transform duration-700 group-hover/refresh:rotate-180", loading && "animate-spin")} />
          </button>
          <button 
            onClick={handleSave}
            disabled={saving}
            className="flex-1 md:flex-none flex items-center justify-center gap-3 px-10 py-4 rounded-2xl bg-primary text-black font-black uppercase tracking-[0.15em] text-xs hover:shadow-[0_8px_32px_rgba(0,186,255,0.3)] hover:brightness-110 transition-all active:scale-95 disabled:opacity-50"
          >
            {saving ? <Loader2 className="w-5 h-5 animate-spin" /> : <Save className="w-5 h-5" />}
            {saving ? 'Persisting...' : 'Save Configuration'}
          </button>
        </div>
      </div>

      {/* Messaging Area */}
      <AnimatePresence>
          {error && (
            <motion.div 
                initial={{ opacity: 0, y: -20 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, scale: 0.95 }}
                className="p-5 rounded-2xl bg-error/10 border border-error/20 flex items-center gap-4 text-error"
            >
                <div className="p-2 bg-error/20 rounded-lg"><AlertCircle className="w-5 h-5" /></div>
                <p className="text-xs font-mono font-bold tracking-tight uppercase">{error}</p>
            </motion.div>
          )}

          {showSuccess && (
            <motion.div 
                initial={{ opacity: 0, y: -20 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, scale: 0.95 }}
                className="p-5 rounded-2xl bg-secondary/10 border border-secondary/20 flex items-center gap-4 text-secondary"
            >
                <div className="p-2 bg-secondary/20 rounded-lg"><CheckCircle2 className="w-5 h-5" /></div>
                <p className="text-xs font-mono font-black uppercase tracking-widest italic">Node parameters updated successfully</p>
            </motion.div>
          )}
      </AnimatePresence>

      {/* Navigation & Controls */}
      <div className="flex flex-col lg:flex-row gap-6 items-start lg:items-center justify-between">
          <div className="flex overflow-x-auto p-1.5 bg-surface-container-lowest/60 rounded-[1.5rem] border border-white/5 w-full md:w-fit backdrop-blur-sm">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => {
                    setActiveTab(tab.id);
                    localStorage.setItem('settings_tab', tab.id);
                }}
                className={cn(
                  "flex-shrink-0 whitespace-nowrap flex items-center gap-3 px-6 py-3 rounded-2xl text-[10px] font-black uppercase tracking-[0.2em] transition-all duration-500",
                  activeTab === tab.id 
                    ? "bg-white text-black shadow-xl scale-105 z-10" 
                    : "text-on-surface-variant hover:text-white hover:bg-white/5"
                )}
              >
                <tab.icon className={cn("w-4 h-4", activeTab === tab.id ? "text-primary" : "text-inherit")} />
                {tab.label}
              </button>
            ))}
          </div>

          <div className="flex items-center gap-4 bg-surface-container-lowest/60 p-2 rounded-[1.5rem] border border-white/5 backdrop-blur-sm">
             <span className="text-[9px] font-black text-on-surface-variant uppercase tracking-widest ml-4 hidden md:inline">Display Unit</span>
             <div className="flex gap-1">
                {(['AUTO', 'FLC', 'LOKI'] as DisplayUnit[]).map((u) => (
                    <button
                        key={u}
                        type="button"
                        onClick={() => setUnit(u)}
                        className={cn(
                            "px-5 py-2.5 rounded-xl text-[10px] font-black tracking-widest transition-all duration-300",
                            unit === u ? "bg-primary text-black" : "text-on-surface-variant hover:text-white"
                        )}
                    >
                        {u}
                    </button>
                ))}
             </div>
          </div>
      </div>

      {/* Main Content Area */}
      <div className="relative min-h-[400px]">
        <AnimatePresence mode="wait">
          <motion.div
            key={activeTab}
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            transition={{ duration: 0.4, ease: [0.16, 1, 0.3, 1] }}
            className="space-y-8"
          >
            {activeTab === 'general' && (
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
                <GlassCard className="lg:col-span-2">
                  <SectionHeader title="Node Identity" subtitle="General connection and branding parameters" icon={Globe} />
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-x-12 gap-y-10">
                    <Input label="Display Name" icon={Activity} explanation="The friendly name for your LSP shown in the UI and to clients." value={formData?.name} onChange={(v) => handleInputChange('name', v)} placeholder="e.g. My Flowgate LSP" onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <Input label="Public Host" icon={Network} explanation="The public host and port for your LSP node. Must be reachable by clients." value={formData?.host} onChange={(v) => handleInputChange('host', v)} placeholder="e.g. 1.2.3.4:9735" onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                  </div>
                </GlassCard>

                <GlassCard>
                   <SectionHeader title="Funding Policy" subtitle="Blockchain confirmation strategy" icon={Clock} />
                   <div className="grid grid-cols-1 gap-10">
                    <Input label="Target Confirmations" icon={Scale} explanation="Target number of blocks for channel funding confirmation." type="number" value={formData?.targetConf} onChange={(v) => handleInputChange('targetConf', parseInt(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <Input label="Min Confirmations" icon={Shield} explanation="Minimum confirmations for inputs used in channel opening." type="number" value={formData?.minConfs} onChange={(v) => handleInputChange('minConfs', parseInt(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                   </div>
                </GlassCard>

                <GlassCard>
                   <SectionHeader title="Infrastructure" subtitle="LND Backend Connectivity" icon={Network} />
                   <div className="grid grid-cols-1 gap-10">
                        <Input label="LND Address" icon={Globe} explanation="gRPC address of your underlying LND node." value={formData?.flnd?.address} onChange={(v) => handleInputChange('flnd.address', v)} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                        <div className="p-4 bg-primary/5 border border-primary/20 rounded-2xl flex items-center gap-4">
                            <Shield className="w-8 h-8 text-primary/40" />
                            <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest leading-relaxed">
                                Macaroons and Certs are managed via <span className="text-primary font-black">environment variables</span> for enhanced security.
                            </p>
                        </div>
                   </div>
                </GlassCard>
              </div>
            )}

            {activeTab === 'jit' && (
              <div className="space-y-8">
                <GlassCard className="!p-0 overflow-hidden">
                    <div className="p-8 md:p-10 bg-gradient-to-r from-primary/10 to-transparent flex flex-col md:flex-row items-start md:items-center justify-between gap-6 border-b border-white/5">
                        <div className="flex items-center gap-5">
                            <div className="w-14 h-14 rounded-[1.5rem] bg-primary text-black flex items-center justify-center shadow-[0_8px_32px_rgba(0,186,255,0.4)]">
                                <Zap className="w-7 h-7 fill-current" />
                            </div>
                            <div>
                                <h3 className="text-xl font-black text-white uppercase italic tracking-tighter">LSPS2 JIT Service</h3>
                                <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-[0.2em] mt-1">On-demand channel creation for incoming HTLCs</p>
                            </div>
                        </div>
                        <div className="flex items-center gap-4 bg-black/40 px-6 py-4 rounded-[2rem] border border-white/10">
                            <span className="text-[10px] font-black text-white uppercase tracking-widest">Protocol Active</span>
                            <Switch checked={formData?.jitEnabled} onChange={(v) => handleInputChange('jitEnabled', v)} />
                        </div>
                    </div>
                    
                    <div className="p-8 md:p-10 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-x-12 gap-y-12">
                      <FinancialInput field="minPaymentSizeMloki" label="Min Payment Threshold" explanation="Minimum payment that triggers a JIT channel." value={formData?.minPaymentSizeMloki} source="MLOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <FinancialInput field="jitMaxPaymentSizeMloki" label="Max Liquidity Limit" explanation="Maximum payment supported for JIT creation." value={formData?.jitMaxPaymentSizeMloki} source="MLOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <FinancialInput field="baseFeeMloki" label="Routing Base Fee" explanation="Fixed cost per HTLC routed." value={formData?.baseFeeMloki} source="MLOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <Input label="PPM Fee Rate" icon={Percent} explanation="Proportional fee in parts-per-million." type="number" step="0.0001" value={formData?.feeRate} onChange={(v) => handleInputChange('feeRate', parseFloat(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <Input label="Permyriad Fee" icon={DollarSign} explanation="Fee charged per myriad for operations." type="number" value={formData?.channelFeePermyriad} onChange={(v) => handleInputChange('channelFeePermyriad', parseInt(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <FinancialInput field="minChannelCapacityLoki" label="Floor Capacity" explanation="Absolute minimum size for any new channel." value={formData?.minChannelCapacityLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    </div>
                </GlassCard>
              </div>
            )}

            {activeTab === 'lsps1' && (
              <div className="space-y-8">
                <GlassCard>
                  <SectionHeader title="Manual Orders" subtitle="Pro-active liquidity purchasing protocol" icon={Shield} />
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-x-12 gap-y-12">
                    <Input label="Funding Confirms" icon={Clock} explanation="Confirmations required for manual LSPS1 orders." type="number" value={formData?.lsps1MinRequiredChannelConfirmations} onChange={(v) => handleInputChange('lsps1MinRequiredChannelConfirmations', parseInt(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <Input label="Max Expiry (Blocks)" icon={Scale} explanation="Maximum allowed channel lifespan in blocks." type="number" value={formData?.lsps1MaxChannelExpiryBlocks} onChange={(v) => handleInputChange('lsps1MaxChannelExpiryBlocks', parseInt(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <Input label="Order TTL (Seconds)" icon={Activity} explanation="Time before a pending order is cancelled." type="number" value={formData?.lsps1OrderExpirySeconds} onChange={(v) => handleInputChange('lsps1OrderExpirySeconds', parseInt(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    
                    <div className="lg:col-span-3 pt-6 border-t border-white/5" />
                    
                    <FinancialInput field="lsps1MinInitialClientBalanceLoki" label="Min Client Stake" explanation="Minimum balance client must fund initially." value={formData?.lsps1MinInitialClientBalanceLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <FinancialInput field="lsps1MaxInitialClientBalanceLoki" label="Max Client Stake" explanation="Maximum balance allowed for client stake." value={formData?.lsps1MaxInitialClientBalanceLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <FinancialInput field="lsps1MinInitialLspBalanceLoki" label="Min LSP Liquidity" explanation="Minimum capacity LSP must provide." value={formData?.lsps1MinInitialLspBalanceLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <FinancialInput field="lsps1MaxInitialLspBalanceLoki" label="Max LSP Liquidity" explanation="Maximum capacity LSP will provide." value={formData?.lsps1MaxInitialLspBalanceLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <FinancialInput field="lsps1MaxChannelBalanceLoki" label="Absolute Max Cap" explanation="Largest possible channel capacity for LSPS1." value={formData?.lsps1MaxChannelBalanceLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                  </div>
                </GlassCard>
              </div>
            )}
          </motion.div>
        </AnimatePresence>
        
        <InfoModal 
            isOpen={!!infoModal}
            onClose={() => setInfoModal(null)}
            label={infoModal?.label || ''}
            explanation={infoModal?.explanation || ''}
        />
        
        {editingField && (
            <CurrencyConverterModal 
                isOpen={!!editingField}
                onClose={() => setEditingField(null)}
                onConfirm={(v) => {
                    handleInputChange(editingField.field, v);
                    setEditingField(null);
                }}
                currentValue={editingField.value}
                sourceUnit={editingField.source}
                label={editingField.label}
                explanation={editingField.explanation}
            />
        )}
      </div>
    </div>
  );
};
