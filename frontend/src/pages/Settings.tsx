import React, { useState, useEffect } from 'react';
import { GlassCard } from '../components/GlassCard';
import { Settings as SettingsIcon, Save, RefreshCw, Zap, Shield, Globe, Loader2, AlertCircle, CheckCircle2, HelpCircle } from 'lucide-react';
import { useSettings } from '../hooks/useSettings';
import { useUnit, type DisplayUnit } from '../contexts/UnitContext';
import { formatAmount, formatInput, cn } from '../lib/utils';
import { motion, AnimatePresence } from 'framer-motion';
import { CurrencyConverterModal } from '../components/CurrencyConverterModal';
import { InfoModal } from '../components/InfoModal';

type Tab = 'general' | 'jit' | 'lsps1';

// Enhanced FinancialInput
const FinancialInput: React.FC<{ 
    field: string;
    label: string; 
    explanation: string; 
    value: any; 
    source: 'LOKI' | 'MLOKI';
    onEdit: (field: string, source: 'LOKI' | 'MLOKI', value: number, label: string, explanation: string) => void;
}> = ({ field, label, explanation, value, source, onEdit }) => (
  <div className="space-y-2">
    <div className="flex justify-between items-center">
      <div className="flex items-center gap-1.5">
        <label className="text-[11px] font-bold text-white uppercase tracking-wider">{label}</label>
        <div className="relative group/help">
            <HelpCircle className="w-5 h-5 text-primary cursor-help" />
            <div className="absolute top-0 left-full ml-4 w-80 p-5 bg-black border border-primary/30 rounded-2xl text-sm text-white/90 leading-relaxed shadow-[0_20px_50px_rgba(0,0,0,0.8)] z-[100] opacity-0 group-hover/help:opacity-100 transition-opacity pointer-events-none whitespace-normal">
                {explanation}
            </div>
        </div>

      </div>
      <button onClick={() => onEdit(field, source, value, label, explanation)} className="text-[10px] text-primary hover:underline font-mono uppercase tracking-widest font-bold">Edit</button>
    </div>
    <div className="flex items-center gap-3 bg-black/40 border border-white/10 rounded-xl p-4">
        <input type="text" readOnly value={formatInput(value)} className="bg-transparent w-full text-white font-mono text-base outline-none" />
        <span className="text-xs text-secondary font-mono font-bold whitespace-nowrap px-3 py-1 bg-secondary/10 rounded-lg border border-secondary/20">
            ≈ {formatAmount(value, 'AUTO', source)}
        </span>
    </div>
  </div>
);

const Input: React.FC<{ label: string; explanation?: string; value: any; onChange: (v: any) => void; type?: string; placeholder?: string; step?: string }> = ({ 
  label, explanation, value, onChange, type = 'text', placeholder, step 
}) => (
  <div className="space-y-2 group">
    <div className="flex items-center gap-1.5">
        <label className="text-[11px] font-mono uppercase tracking-[0.2em] text-on-surface-variant group-focus-within:text-primary transition-colors">
        {label}
        </label>
        {explanation && (
            <div className="group relative flex items-center">
                <HelpCircle className="w-5 h-5 text-primary cursor-help" />
                <div className="absolute left-full ml-4 w-96 p-6 bg-black border border-primary/30 rounded-2xl text-sm text-white/90 leading-relaxed shadow-[0_20px_50px_rgba(0,0,0,0.8)] z-50 opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none">
                    {explanation}
                </div>
            </div>

        )}
    </div>
    <input 
      type={type}
      step={step}
      value={value ?? ''}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
      className="w-full bg-black/40 border border-white/10 rounded-xl px-4 py-3 text-sm text-white font-mono placeholder:text-white/20 focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/20 transition-all"
    />
  </div>
);


const Switch: React.FC<{ checked: boolean; onChange: (v: boolean) => void }> = ({ checked, onChange }) => (
  <button 
    onClick={() => onChange(!checked)}
    className={cn(
        "w-12 h-6 rounded-full transition-all duration-300 relative",
        checked ? "bg-primary" : "bg-white/10"
    )}
  >
    <div className={cn(
        "absolute top-1 left-1 w-4 h-4 rounded-full bg-white transition-all duration-300",
        checked ? "translate-x-6" : "translate-x-0"
    )} />
  </button>
);

export const Settings: React.FC = () => {
  const { config, loading, saving, error, refreshConfig, updateConfig } = useSettings();
  const { unit, setUnit } = useUnit();
  const [activeTab, setActiveTab] = useState<Tab>('general');
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
      const next = { ...prev };
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
      <div className="h-[60vh] flex items-center justify-center">
        <Loader2 className="w-8 h-8 text-primary animate-spin" />
      </div>
    );
  }

  const tabs: { id: Tab; label: string; icon: any }[] = [
    { id: 'general', label: 'General', icon: Globe },
    { id: 'jit', label: 'JIT Channels', icon: Zap },
    { id: 'lsps1', label: 'LSPS1 Protocol', icon: Shield },
  ];

  return (
    <div className="p-3 md:p-10 max-w-5xl mx-auto space-y-6 md:space-y-8 animate-in fade-in duration-1000 pb-32">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:justify-between md:items-end gap-6">
        <div className="space-y-2">
          <h1 className="text-2xl md:text-4xl font-black tracking-tighter text-white uppercase italic leading-none flex items-center gap-3">
             <SettingsIcon className="w-7 h-7 md:w-10 md:h-10 text-primary" />
             Node Settings
          </h1>
          <p className="text-on-surface-variant font-mono text-[9px] md:text-xs tracking-widest uppercase mt-1">
            Manage LSP node parameters and protocol behavior
          </p>
        </div>
        <div className="flex gap-3 w-full md:w-auto mt-2 md:mt-0">
          <button 
            onClick={refreshConfig}
            className="p-3 md:p-3.5 rounded-xl bg-white/5 border border-white/10 text-white hover:bg-white/10 transition-all active:scale-95 flex items-center justify-center aspect-square shrink-0"
            title="Refresh Config"
          >
            <RefreshCw className={cn("w-4 h-4 md:w-5 md:h-5", loading && "animate-spin")} />
          </button>
          <button 
            onClick={handleSave}
            disabled={saving}
            className="flex-1 md:flex-none flex items-center justify-center gap-2 px-6 py-3 md:py-3.5 rounded-xl bg-primary text-black font-black uppercase tracking-widest text-[10px] md:text-xs hover:brightness-110 transition-all active:scale-95 disabled:opacity-50 disabled:pointer-events-none shadow-[0_4px_24px_rgba(0,186,255,0.25)]"
          >
            {saving ? <Loader2 className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
            {saving ? 'Saving...' : 'Save Changes'}
          </button>
        </div>
      </div>

      {error && (
        <div className="p-4 rounded-xl bg-error/10 border border-error/20 flex items-center gap-3 text-error animate-in slide-in-from-top-2">
            <AlertCircle className="w-5 h-5" />
            <p className="text-xs font-mono font-medium">{error}</p>
        </div>
      )}

      {showSuccess && (
        <div className="p-4 rounded-xl bg-secondary/10 border border-secondary/20 flex items-center gap-3 text-secondary animate-in slide-in-from-top-2">
            <CheckCircle2 className="w-5 h-5" />
            <p className="text-xs font-mono font-medium lowercase italic tracking-tight">Configuration persisted successfully</p>
        </div>
      )}

      {/* Tabs */}
      <div className="flex overflow-x-auto p-1 bg-white/5 rounded-xl md:rounded-2xl border border-white/5 w-full md:w-fit custom-scrollbar">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={cn(
              "flex-shrink-0 whitespace-nowrap flex items-center gap-2 px-5 py-2.5 rounded-lg md:rounded-xl text-[9px] md:text-[10px] font-bold uppercase tracking-widest transition-all",
              activeTab === tab.id ? "bg-white/10 text-white border border-white/10" : "text-on-surface-variant hover:text-white"
            )}
          >
            <tab.icon className={cn("w-3.5 h-3.5 md:w-4 md:h-4", activeTab === tab.id ? "text-primary" : "text-inherit")} />
            {tab.label}
          </button>
        ))}
      </div>

      {/* Content */}
      <div className="relative">
        <AnimatePresence mode="wait">
          <motion.div
            key={activeTab}
            initial={{ opacity: 0, x: 20 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -20 }}
            transition={{ duration: 0.3 }}
            className="space-y-6"
          >
            {activeTab === 'general' && (
              <>
                <GlassCard className="space-y-8">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                    <Input 
                      label="Display Name" 
                      explanation="The friendly name for your LSP shown in the UI."
                      value={formData?.name} 
                      onChange={(v) => handleInputChange('name', v)}
                      placeholder="e.g. My Flowgate LSP"
                    />
                    <Input 
                      label="Public Host" 
                      explanation="The public host and port for your LSP node."
                      value={formData?.host} 
                      onChange={(v) => handleInputChange('host', v)}
                      placeholder="e.g. lsp.example.com:9735"
                    />
                    <Input 
                      label="Target Confirmations" 
                      explanation="The target number of confirmations for channel funding."
                      type="number"
                      value={formData?.targetConf} 
                      onChange={(v) => handleInputChange('targetConf', parseInt(v))}
                    />
                    <Input 
                      label="Min Confirmations" 
                      explanation="The minimum number of confirmations required for a channel."
                      type="number"
                      value={formData?.minConfs} 
                      onChange={(v) => handleInputChange('minConfs', parseInt(v))}
                    />
                  </div>
                </GlassCard>

                <GlassCard className="space-y-6">
                  <div>
                    <h3 className="text-sm font-bold text-white uppercase italic tracking-tighter mb-1">Display Settings</h3>
                    <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest">Configure how financial values are displayed</p>
                  </div>
                  
                  <div className="flex flex-col md:flex-row md:items-center justify-between p-4 bg-white/5 rounded-2xl border border-white/10 gap-4">
                    <div className="space-y-1">
                        <h4 className="text-xs font-bold text-white uppercase tracking-tight">Preferred Currency Unit</h4>
                        <p className="text-[9px] font-mono text-on-surface-variant">Default is AUTO. Switch to FLC or Loki for fixed displays.</p>
                    </div>
                    <div className="flex p-1 bg-black/40 rounded-xl border border-white/5 w-fit">
                        {(['AUTO', 'FLC', 'LOKI'] as DisplayUnit[]).map((u) => (
                            <button
                                key={u}
                                onClick={() => setUnit(u)}
                                className={cn(
                                    "px-6 py-2 rounded-lg text-[10px] font-black tracking-widest transition-all",
                                    unit === u ? "bg-primary text-black" : "text-on-surface-variant hover:text-white"
                                )}
                            >
                                {u}
                            </button>
                        ))}
                    </div>
                  </div>
                </GlassCard>
              </>
            )}

            {activeTab === 'jit' && (
              <GlassCard className="space-y-8">
                <div className="flex items-center justify-between p-4 bg-white/5 rounded-2xl border border-white/10 mb-8">
                    <div>
                        <h4 className="text-sm font-bold text-white uppercase italic tracking-tighter">Enable JIT Channels</h4>
                        <p className="text-[10px] font-mono text-on-surface-variant">Allow zero-conf JIT channel creation for incoming payments</p>
                    </div>
                    <Switch 
                        checked={formData?.jitEnabled} 
                        onChange={(v) => handleInputChange('jitEnabled', v)} 
                    />
                </div>
                
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
                  <FinancialInput field="minPaymentSizeMloki" label="Min Payment Size" explanation="Minimum payment size in mLoki for JIT channel creation." value={formData?.minPaymentSizeMloki} source="MLOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} />
                  <FinancialInput field="jitMaxPaymentSizeMloki" label="Max Payment Size" explanation="Maximum payment size in mLoki for JIT channel creation." value={formData?.jitMaxPaymentSizeMloki} source="MLOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} />
                  <FinancialInput field="baseFeeMloki" label="Base Fee" explanation="Fixed base fee in mLoki per payment." value={formData?.baseFeeMloki} source="MLOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} />
                  <Input 
                    label="Fee Rate" 
                    type="number"
                    step="0.0001"
                    value={formData?.feeRate} 
                    onChange={(v) => handleInputChange('feeRate', parseFloat(v))}
                  />
                  <Input 
                    label="Channel Fee (Permyriad)" 
                    type="number"
                    value={formData?.channelFeePermyriad} 
                    onChange={(v) => handleInputChange('channelFeePermyriad', parseInt(v))}
                  />
                  <FinancialInput field="minChannelCapacityLoki" label="Min Channel Capacity" explanation="Minimum total capacity for a channel (Loki)." value={formData?.minChannelCapacityLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} />
                </div>
              </GlassCard>
            )}

            {activeTab === 'lsps1' && (
              <GlassCard className="space-y-8">
                 <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
                  <Input 
                    label="Min Channel Confirms" 
                    type="number"
                    value={formData?.lsps1MinRequiredChannelConfirmations} 
                    onChange={(v) => handleInputChange('lsps1MinRequiredChannelConfirmations', parseInt(v))}
                  />
                  <Input 
                    label="Max Channel Expiry (Blocks)" 
                    type="number"
                    value={formData?.lsps1MaxChannelExpiryBlocks} 
                    onChange={(v) => handleInputChange('lsps1MaxChannelExpiryBlocks', parseInt(v))}
                  />
                  <Input 
                    label="Order Expiry (Seconds)" 
                    type="number"
                    value={formData?.lsps1OrderExpirySeconds} 
                    onChange={(v) => handleInputChange('lsps1OrderExpirySeconds', parseInt(v))}
                  />
                  <FinancialInput field="lsps1MinInitialClientBalanceLoki" label="Min Initial Client Balance" explanation="Minimum balance the client is required to maintain." value={formData?.lsps1MinInitialClientBalanceLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} />
                  <FinancialInput field="lsps1MaxInitialClientBalanceLoki" label="Max Initial Client Balance" explanation="Maximum balance allowed for the client." value={formData?.lsps1MaxInitialClientBalanceLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} />
                  <FinancialInput field="lsps1MinInitialLspBalanceLoki" label="Min Initial LSP Balance" explanation="Minimum initial liquidity provided by the LSP." value={formData?.lsps1MinInitialLspBalanceLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} />
                  <FinancialInput field="lsps1MaxInitialLspBalanceLoki" label="Max Initial LSP Balance" explanation="Maximum initial liquidity provided by the LSP." value={formData?.lsps1MaxInitialLspBalanceLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} />
                </div>
              </GlassCard>
            )}
          </motion.div>
        </AnimatePresence>
        {infoModal && (
            <InfoModal 
                isOpen={!!infoModal}
                onClose={() => setInfoModal(null)}
                label={infoModal.label}
                explanation={infoModal.explanation}
            />
        )}
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
