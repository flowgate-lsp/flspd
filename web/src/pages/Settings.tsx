import React, { useState, useEffect } from 'react';
import { GlassCard } from '../components/GlassCard';
import { DualRangeSlider } from '../components/DualRangeSlider';
import {
    Settings as SettingsIcon, Save, RefreshCw, Zap, Shield, Globe,
    Loader2, AlertCircle, CheckCircle2, HelpCircle, ArrowRightLeft,
    Network, Clock, Percent, Scale, Activity, SlidersHorizontal, Sparkles,
    Megaphone, RadioTower
} from 'lucide-react';
import { useSettings } from '../hooks/useSettings';
import { useUnit, type DisplayUnit } from '../contexts/UnitContext';
import { formatAmount, formatInput, cn } from '../lib/utils';
import { computeOpeningFeeLoki } from '../lib/feeCalc';
import { motion, AnimatePresence } from 'framer-motion';
import { CurrencyConverterModal } from '../components/CurrencyConverterModal';
import { InfoModal } from '../components/InfoModal';
import {
  toAnnounceForm, fromAnnounceForm, announceHints, charCount,
  TITLE_MAX, CONTENT_MAX, type AnnounceForm,
} from '../lib/announce';

type Tab = 'connect' | 'business' | 'advanced';

// Profile-controlled fields (config/tiered.go's applyProfile): switching
// the profile dropdown must clear these back to their "unset" sentinel
// (0 / null) client-side, or the backend's "only fill in fields still at
// their zero value" precedence rule (an explicit value always wins) would
// see the PREVIOUS profile's already-expanded values and never apply the
// newly selected one.
const PROFILE_CONTROLLED_FIELDS: Record<string, any> = {
  targetConf: 0,
  minConfs: null,
  timeLockDelta: 0,
  maxInactiveDuration: 0,
  lsps1OrderExpirySeconds: null,
  jitValidUntilSeconds: 0,
};

const PROFILE_OPTIONS: { id: string; label: string; blurb: string }[] = [
  { id: 'conservative', label: 'Conservative', blurb: 'More confirmations, longer safety margins, shorter-lived offers.' },
  { id: 'balanced', label: 'Balanced', blurb: 'A reasonable default for most deployments.' },
  { id: 'aggressive', label: 'Aggressive', blurb: 'Prioritizes speed/UX — allows 0-conf, shorter CLTV delta.' },
];

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
        <div className="w-full bg-surface-container-lowest/80 border border-outline-variant rounded-2xl p-4 flex flex-col gap-2 group-focus-within/input:border-primary/40 group-focus-within/input:ring-4 group-focus-within/input:ring-primary/5 transition-all duration-500">
            <div className="flex items-baseline justify-between gap-4">
                <span className="text-xl md:text-2xl font-mono text-white tracking-tighter truncate selection:bg-primary/30">
                    {formatInput(value)}
                </span>
                <span className="text-[9px] font-black text-on-surface-variant uppercase tracking-widest bg-white/5 px-2 py-0.5 rounded-md border border-outline-variant shrink-0">
                    {source}
                </span>
            </div>

            <div className="flex items-center gap-2.5 pt-2 border-t border-outline-variant">
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
            "w-full bg-surface-container-lowest/80 border border-outline-variant rounded-2xl py-4 pr-4 text-sm text-white font-mono placeholder:text-white/10 focus:outline-none focus:border-primary/40 focus:ring-4 focus:ring-primary/5 transition-all duration-500",
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
        checked ? "bg-primary shadow-[0_0_20px_rgba(0,186,255,0.3)]" : "bg-white/5 border border-outline"
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

// derives the tiered blocks (opening_fee, routing_fee, channel_size) from
// whatever flat fields are currently present, so a deployment that hasn't
// touched the new schema yet still sees its actual current settings the
// first time it opens the Business tab, rather than blank/default values.
function withTieredDefaults(cfg: any): any {
  const next = { ...cfg };
  if (!next.opening_fee) {
    next.opening_fee = {
      rate_percent: (next.channelFeePermyriad ?? 0) / 100,
      min_loki: Math.floor((next.channelMinimumFeeMloki ?? 0) / 1000),
    };
  }
  if (!next.routing_fee) {
    next.routing_fee = {
      ppm: Math.round((next.feeRate ?? 0) * 1_000_000),
      base_loki: Math.floor((next.baseFeeMloki ?? 0) / 1000),
    };
  }
  if (!next.channel_size) {
    next.channel_size = {
      min_loki: next.minChannelCapacityLoki ?? 20000,
      max_loki: next.lsps1MaxChannelBalanceLoki ?? 100_000_000,
      padding_loki: next.additionalChannelCapacity ?? 0,
    };
  }
  return next;
}

export const Settings: React.FC = () => {
  const { config, announceRelays, loading, saving, error, refreshConfig, updateConfig } = useSettings();
  const [announceForm, setAnnounceForm] = useState<AnnounceForm>(toAnnounceForm(null));
  const { unit, setUnit } = useUnit();
  const [activeTab, setActiveTab] = useState<Tab>((localStorage.getItem('settings_tab') as Tab) || 'connect');
  const [formData, setFormData] = useState<any>(null);
  const [showSuccess, setShowSuccess] = useState(false);
  const [editingField, setEditingField] = useState<{ field: string; source: 'LOKI' | 'MLOKI'; value: number; label: string; explanation: string } | null>(null);
  const [infoModal, setInfoModal] = useState<{ label: string; explanation: string } | null>(null);
  const [previewPaymentLoki, setPreviewPaymentLoki] = useState(1_000_000);

  useEffect(() => {
    if (config) {
      setFormData(withTieredDefaults(config));
      setAnnounceForm(toAnnounceForm(config.announce));
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

  const setAnnounce = (patch: Partial<AnnounceForm>) =>
    setAnnounceForm((prev) => ({ ...prev, ...patch }));

  const handleProfileChange = (newProfile: string) => {
    setFormData((prev: any) => ({
      ...prev,
      profile: newProfile,
      ...PROFILE_CONTROLLED_FIELDS,
    }));
  };

  const handleSave = async () => {
    const success = await updateConfig({ ...formData, announce: fromAnnounceForm(announceForm) });
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
    { id: 'connect', label: 'Connect', icon: Globe },
    { id: 'business', label: 'Business Policy', icon: Sparkles },
    { id: 'advanced', label: 'Advanced', icon: SlidersHorizontal },
  ];

  const openingFee = formData?.opening_fee ?? { rate_percent: 0, min_loki: 0 };
  const channelSize = formData?.channel_size ?? { min_loki: 20000, max_loki: 100_000_000, padding_loki: 0 };
  const previewFeeLoki = computeOpeningFeeLoki(previewPaymentLoki, openingFee.rate_percent ?? 0, openingFee.min_loki ?? 0);
  const feeExampleSizes = [100_000, 1_000_000, 10_000_000];

  return (
    <div className="p-4 md:p-10 max-w-6xl mx-auto space-y-8 md:space-y-12 animate-in fade-in duration-1000 pb-40">
      {/* Header Section */}
      <div className="flex flex-col md:flex-row md:justify-between md:items-center gap-8 bg-surface-variant/20 p-6 md:p-8 rounded-[2rem] border border-outline-variant backdrop-blur-md relative overflow-hidden group">
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
            className="group/refresh p-4 rounded-2xl bg-white/5 border border-outline text-white hover:bg-white/10 transition-all active:scale-95 flex items-center justify-center"
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
          <div className="flex overflow-x-auto p-1.5 bg-surface-container-lowest/60 rounded-[1.5rem] border border-outline-variant w-full md:w-fit backdrop-blur-sm">
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

          <div className="flex items-center gap-4 bg-surface-container-lowest/60 p-2 rounded-[1.5rem] border border-outline-variant backdrop-blur-sm">
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
            {activeTab === 'connect' && (
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
                <GlassCard className="lg:col-span-2">
                  <SectionHeader title="Node Identity" subtitle="General connection and branding parameters" icon={Globe} />
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-x-12 gap-y-10">
                    <Input label="Display Name" icon={Activity} explanation="The friendly name for your LSP shown in the UI and to clients." value={formData?.name} onChange={(v) => handleInputChange('name', v)} placeholder="e.g. My Flowgate LSP" onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <Input label="Public Host" icon={Network} explanation="The public host and port for your LSP node. Must be reachable by clients." value={formData?.host} onChange={(v) => handleInputChange('host', v)} placeholder="e.g. 1.2.3.4:9735" onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                  </div>
                </GlassCard>

                <GlassCard className="lg:col-span-2">
                  <SectionHeader title="Announce on Nostr" subtitle="NIP-LSP announcement, signed by the node key" icon={Megaphone} />
                  <div className="space-y-10">
                    <div className="flex items-center justify-between gap-6 p-4 bg-white/5 border border-outline-variant rounded-2xl">
                      <div>
                        <p className="text-sm font-black text-white">Announce this node</p>
                        <p className="text-[11px] text-on-surface-variant mt-1 leading-relaxed">
                          Publishes the LSPS offer (protocols, LSPS1 options, the LSPS2 fee menu) so wallets can find this LSP. The offer always comes from the live LSPS answers; the fields below only describe the LSP and say where to reach it.
                        </p>
                      </div>
                      <Switch checked={announceForm.enabled} onChange={(v) => setAnnounce({ enabled: v })} />
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-2 gap-x-12 gap-y-10">
                      <div>
                        <Input label="Title" icon={Activity} explanation={`The LSP's name, one line, at most ${TITLE_MAX} characters. Wallets show it as a claim, next to the node id.`} value={announceForm.title} onChange={(v) => setAnnounce({ title: v })} placeholder="e.g. My Flowgate LSP" onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                        <p className={cn("text-[10px] font-mono mt-2 px-1", charCount(announceForm.title) > TITLE_MAX ? "text-error" : "text-on-surface-variant/60")}>{charCount(announceForm.title)} / {TITLE_MAX}</p>
                      </div>
                      <Input label="Website" icon={Globe} explanation="An https URL for the LSP." value={announceForm.website} onChange={(v) => setAnnounce({ website: v })} placeholder="https://lsp.example.com" onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <Input label="Logo URL" icon={Globe} explanation="An https URL of a logo. Wallets load it only when the user asks." value={announceForm.image} onChange={(v) => setAnnounce({ image: v })} placeholder="https://lsp.example.com/logo.png" onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <Input label="Languages" icon={Globe} explanation="ISO-639-1 codes, comma-separated, e.g. en, fr." value={announceForm.languages} onChange={(v) => setAnnounce({ languages: v })} placeholder="en, fr" onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <Input label="Operator Pubkey" icon={Shield} explanation="Your own Nostr pubkey (hex or npub), so buyers can reach the operator." value={announceForm.operatorPubkey} onChange={(v) => setAnnounce({ operatorPubkey: v })} placeholder="npub1... or 64 hex" onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <Input label="Operator Relay" icon={RadioTower} explanation="A wss:// relay where your operator pubkey can be found. Required with the pubkey." value={announceForm.operatorRelay} onChange={(v) => setAnnounce({ operatorRelay: v })} placeholder="wss://relay.example.com" onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    </div>

                    <div className="space-y-3">
                      <label className="text-[10px] font-black text-on-surface-variant uppercase tracking-[0.2em] px-1">Description</label>
                      <textarea
                        value={announceForm.content}
                        onChange={(e) => setAnnounce({ content: e.target.value })}
                        rows={3}
                        placeholder="Inbound channels and just-in-time channels."
                        className="w-full bg-surface-container-lowest/80 border border-outline-variant rounded-2xl p-4 text-sm text-white font-mono placeholder:text-white/10 focus:outline-none focus:border-primary/40 focus:ring-4 focus:ring-primary/5 transition-all duration-500"
                      />
                      <p className={cn("text-[10px] font-mono px-1", charCount(announceForm.content) > CONTENT_MAX ? "text-error" : "text-on-surface-variant/60")}>{charCount(announceForm.content)} / {CONTENT_MAX}</p>
                    </div>

                    <div className="space-y-3">
                      <label className="text-[10px] font-black text-on-surface-variant uppercase tracking-[0.2em] px-1">Node Addresses</label>
                      <textarea
                        value={announceForm.addrs}
                        onChange={(e) => setAnnounce({ addrs: e.target.value })}
                        rows={3}
                        placeholder={"<56 characters>.onion:9735\n203.0.113.7:9735"}
                        className="w-full bg-surface-container-lowest/80 border border-outline-variant rounded-2xl p-4 text-sm text-white font-mono placeholder:text-white/10 focus:outline-none focus:border-primary/40 focus:ring-4 focus:ring-primary/5 transition-all duration-500"
                      />
                      <p className="text-[11px] text-on-surface-variant px-1 leading-relaxed">
                        One host:port per line: IPv4, bracketed IPv6 or onion. Announced exactly as written, in this order; at least one is needed. Public Host above is not added. An onion-only list keeps the node's location private.
                      </p>
                    </div>

                    <div className="p-4 bg-primary/5 border border-primary/20 rounded-2xl flex items-start gap-4">
                      <RadioTower className="w-6 h-6 text-primary/60 shrink-0" />
                      <div className="text-[11px] text-on-surface-variant leading-relaxed">
                        <p className="font-black text-white mb-1">Relays (NOSTR_ANNOUNCE_RELAYS)</p>
                        {announceRelays.length > 0 ? (
                          <ul className="font-mono space-y-0.5">{announceRelays.map((r) => <li key={r}>{r}</li>)}</ul>
                        ) : (
                          <p>None set. Announcing needs NOSTR_ANNOUNCE_RELAYS in flspd's environment, and an flnd macaroon with signer:generate.</p>
                        )}
                      </div>
                    </div>

                    {announceHints(announceForm).length > 0 && (
                      <ul className="p-4 rounded-2xl bg-tertiary/10 border border-tertiary/20 text-[11px] text-tertiary space-y-1">
                        {announceHints(announceForm).map((h) => <li key={h}>{h}</li>)}
                      </ul>
                    )}
                  </div>
                </GlassCard>

                <GlassCard className="lg:col-span-2">
                   <SectionHeader title="Infrastructure" subtitle="flnd Backend Connectivity" icon={Network} />
                   <div className="grid grid-cols-1 gap-10">
                        <Input label="flnd Address" icon={Globe} explanation="gRPC address of your underlying flnd node." value={formData?.flnd?.address} onChange={(v) => handleInputChange('flnd.address', v)} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                        <div className="p-4 bg-primary/5 border border-primary/20 rounded-2xl flex items-center gap-4">
                            <Shield className="w-8 h-8 text-primary/40" />
                            <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest leading-relaxed">
                                The LSP private key, TLS cert, and macaroon are configured via <span className="text-primary font-black">from_env / from_file references</span> in nodes.yaml, not through this dashboard — see SecretRef in config/secretref.go.
                            </p>
                        </div>
                   </div>
                </GlassCard>
              </div>
            )}

            {activeTab === 'business' && (
              <div className="space-y-8">
                {/* Risk profile */}
                <GlassCard>
                  <SectionHeader title="Risk Profile" subtitle="Confirmation / timelock / expiry bundle" icon={Shield} />
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    {PROFILE_OPTIONS.map((p) => (
                      <button
                        key={p.id}
                        type="button"
                        onClick={() => handleProfileChange(p.id)}
                        className={cn(
                          "text-left p-5 rounded-2xl border transition-all duration-300 space-y-2",
                          formData?.profile === p.id
                            ? "bg-primary/10 border-primary/40 ring-4 ring-primary/5"
                            : "bg-surface-container-lowest/60 border-outline-variant hover:border-outline"
                        )}
                      >
                        <p className="text-sm font-black text-white uppercase tracking-tight">{p.label}</p>
                        <p className="text-[10px] font-mono text-on-surface-variant leading-relaxed">{p.blurb}</p>
                      </button>
                    ))}
                  </div>
                  {formData?.profile && (
                    <div className="mt-6 p-5 bg-surface-container-low/60 rounded-2xl border border-outline-variant grid grid-cols-2 md:grid-cols-3 gap-4">
                      {[
                        ['Target Conf', formData?.targetConf],
                        ['Min Conf', formData?.minConfs],
                        ['CLTV Delta', formData?.timeLockDelta],
                        ['Max Inactive (s)', formData?.maxInactiveDuration],
                        ['LSPS1 Order TTL (s)', formData?.lsps1OrderExpirySeconds],
                        ['JIT Valid Until (s)', formData?.jitValidUntilSeconds],
                      ].map(([label, value]) => (
                        <div key={label as string}>
                          <p className="text-[9px] font-black text-on-surface-variant uppercase tracking-widest">{label}</p>
                          <p className="text-sm font-mono text-secondary font-bold">{value ?? '—'}</p>
                        </div>
                      ))}
                      <p className="col-span-2 md:col-span-3 text-[9px] font-mono text-on-surface-variant/60 uppercase tracking-wider pt-2 border-t border-outline-variant">
                        Read-only expansion of the selected profile's effective values, as currently loaded. Save and reload to preview a newly selected profile.
                      </p>
                    </div>
                  )}
                </GlassCard>

                {/* Opening fee */}
                <GlassCard>
                  <SectionHeader title="Channel Opening Fee" subtitle="What clients pay for a new/JIT channel" icon={Percent} />
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-x-12 gap-y-10">
                    <Input
                      label="Rate" icon={Percent}
                      explanation="Proportional fee, as a percentage of the payment size."
                      type="number" step="0.01"
                      value={openingFee.rate_percent}
                      onChange={(v) => handleInputChange('opening_fee.rate_percent', parseFloat(v) || 0)}
                      onInfo={(l, e) => setInfoModal({ label: l, explanation: e })}
                    />
                    <Input
                      label="Floor (loki)" icon={Scale}
                      explanation="Minimum fee charged, regardless of the proportional rate."
                      type="number"
                      value={openingFee.min_loki}
                      onChange={(v) => handleInputChange('opening_fee.min_loki', parseInt(v) || 0)}
                      onInfo={(l, e) => setInfoModal({ label: l, explanation: e })}
                    />
                  </div>

                  <div className="mt-8 p-6 bg-primary/5 border border-primary/20 rounded-2xl space-y-4">
                    <div className="flex items-center gap-3">
                      <Input
                        label="Preview payment size (loki)" icon={Activity}
                        explanation="A hypothetical payment size, to see what fee it would incur."
                        type="number"
                        value={previewPaymentLoki}
                        onChange={(v) => setPreviewPaymentLoki(parseInt(v) || 0)}
                        onInfo={(l, e) => setInfoModal({ label: l, explanation: e })}
                      />
                    </div>
                    <p className="text-sm font-mono text-white leading-relaxed">
                      For a payment of <span className="text-secondary font-bold">{previewPaymentLoki.toLocaleString()} loki</span>, the opening fee would be{' '}
                      <span className="text-primary font-black">{previewFeeLoki.toLocaleString()} loki</span>{' '}
                      ({(openingFee.rate_percent ?? 0)}% proportional, {(openingFee.min_loki ?? 0).toLocaleString()} loki floor).
                    </p>
                    <table className="w-full text-[11px] font-mono">
                      <thead>
                        <tr className="text-on-surface-variant uppercase tracking-widest text-[9px]">
                          <th className="text-left py-1">Payment (loki)</th>
                          <th className="text-right py-1">Fee (loki)</th>
                        </tr>
                      </thead>
                      <tbody>
                        {feeExampleSizes.map((size) => (
                          <tr key={size} className="border-t border-outline-variant">
                            <td className="py-2 text-white">{size.toLocaleString()}</td>
                            <td className="py-2 text-right text-secondary font-bold">
                              {computeOpeningFeeLoki(size, openingFee.rate_percent ?? 0, openingFee.min_loki ?? 0).toLocaleString()}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </GlassCard>

                {/* Channel size */}
                <GlassCard>
                  <SectionHeader title="Channel Size" subtitle="Bounds applied to LSPS1 and JIT channels" icon={Scale} />
                  <DualRangeSlider
                    min={20_000}
                    max={1_000_000_000}
                    step={1000}
                    valueMin={channelSize.min_loki ?? 20000}
                    valueMax={channelSize.max_loki ?? 100_000_000}
                    onChangeMin={(v) => handleInputChange('channel_size.min_loki', v)}
                    onChangeMax={(v) => handleInputChange('channel_size.max_loki', v)}
                    formatValue={(v) => `${v.toLocaleString()} loki`}
                  />
                  <div className="mt-8 grid grid-cols-1 md:grid-cols-2 gap-x-12 gap-y-10">
                    <Input
                      label="Min Capacity (loki)" icon={Scale}
                      explanation="Absolute minimum size for any new channel."
                      type="number"
                      value={channelSize.min_loki}
                      onChange={(v) => handleInputChange('channel_size.min_loki', parseInt(v) || 0)}
                      onInfo={(l, e) => setInfoModal({ label: l, explanation: e })}
                    />
                    <Input
                      label="Max Capacity (loki)" icon={Scale}
                      explanation="Largest channel capacity this LSP will offer."
                      type="number"
                      value={channelSize.max_loki}
                      onChange={(v) => handleInputChange('channel_size.max_loki', parseInt(v) || 0)}
                      onInfo={(l, e) => setInfoModal({ label: l, explanation: e })}
                    />
                    <Input
                      label="JIT Padding (loki)" icon={Zap}
                      explanation="Headroom added on top of the incoming payment when opening a JIT channel."
                      type="number"
                      value={channelSize.padding_loki}
                      onChange={(v) => handleInputChange('channel_size.padding_loki', parseInt(v) || 0)}
                      onInfo={(l, e) => setInfoModal({ label: l, explanation: e })}
                    />
                  </div>
                </GlassCard>

                {/* Product toggles */}
                <GlassCard>
                  <SectionHeader title="Products" subtitle="Which protocols this LSP offers" icon={Zap} />
                  <div className="space-y-6">
                    <div className="flex items-center justify-between p-5 bg-surface-container-lowest/60 rounded-2xl border border-outline-variant">
                      <div>
                        <p className="text-sm font-black text-white uppercase tracking-tight">LSPS2 / JIT Channels</p>
                        <p className="text-[10px] font-mono text-on-surface-variant mt-1">On-demand channel creation for incoming HTLCs.</p>
                      </div>
                      <Switch checked={!!formData?.jitEnabled} onChange={(v) => handleInputChange('jitEnabled', v)} />
                    </div>
                    <div className="flex items-center justify-between p-5 bg-surface-container-lowest/60 rounded-2xl border border-outline-variant">
                      <div>
                        <p className="text-sm font-black text-white uppercase tracking-tight">LSPS1 Zero Channel Reserve</p>
                        <p className="text-[10px] font-mono text-on-surface-variant mt-1">Offer channels without requiring a client-side reserve.</p>
                      </div>
                      <Switch checked={!!formData?.lsps1SupportsZeroChannelReserve} onChange={(v) => handleInputChange('lsps1SupportsZeroChannelReserve', v)} />
                    </div>
                  </div>
                </GlassCard>
              </div>
            )}

            {activeTab === 'advanced' && (
              <div className="space-y-8">
                <GlassCard className="!p-6 bg-tertiary/5 border-tertiary/20">
                  <p className="text-[10px] font-mono text-on-surface-variant leading-relaxed">
                    <span className="text-tertiary font-black uppercase">Note:</span> every field below is still fully
                    supported for backward compatibility. If a Business Policy block above (opening fee, channel size,
                    or a risk profile) is also configured, that block takes precedence over the corresponding
                    individual field here — see BACKLOG.md's T5-2..T5-5 for the exact mapping.
                  </p>
                </GlassCard>

                <GlassCard>
                  <SectionHeader title="Funding Policy" subtitle="Blockchain confirmation strategy" icon={Clock} />
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-x-12 gap-y-10">
                    <Input label="Target Confirmations" icon={Scale} explanation="Target number of blocks for channel funding confirmation." type="number" value={formData?.targetConf} onChange={(v) => handleInputChange('targetConf', parseInt(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <Input label="Min Confirmations" icon={Shield} explanation="Minimum confirmations for inputs used in channel opening." type="number" value={formData?.minConfs} onChange={(v) => handleInputChange('minConfs', parseInt(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <Input label="Timelock Delta" icon={Clock} explanation="Minimum timelock delta required for the opened channel." type="number" value={formData?.timeLockDelta} onChange={(v) => handleInputChange('timeLockDelta', parseInt(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <Input label="Max Inactive Duration (s)" icon={Activity} explanation="Duration in seconds used to calculate the MinLifetime block height." type="number" value={formData?.maxInactiveDuration} onChange={(v) => handleInputChange('maxInactiveDuration', parseInt(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                  </div>
                </GlassCard>

                <GlassCard>
                    <SectionHeader title="LSPS2 / JIT" subtitle="On-demand channel creation for incoming HTLCs" icon={Zap} />
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-x-12 gap-y-12">
                      <FinancialInput field="minPaymentSizeMloki" label="Min Payment Threshold" explanation="Minimum payment that triggers a JIT channel." value={formData?.minPaymentSizeMloki} source="MLOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <FinancialInput field="jitMaxPaymentSizeMloki" label="Max Liquidity Limit" explanation="Maximum payment supported for JIT creation." value={formData?.jitMaxPaymentSizeMloki} source="MLOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <FinancialInput field="baseFeeMloki" label="Routing Base Fee" explanation="Fixed cost per HTLC routed." value={formData?.baseFeeMloki} source="MLOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <Input label="PPM Fee Rate" icon={Percent} explanation="Proportional routing fee, as a bare fraction (0.000001 = 1ppm)." type="number" step="0.0001" value={formData?.feeRate} onChange={(v) => handleInputChange('feeRate', parseFloat(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <Input label="Permyriad Fee" icon={Percent} explanation="Opening fee rate, per 10,000 loki (permyriad)." type="number" value={formData?.channelFeePermyriad} onChange={(v) => handleInputChange('channelFeePermyriad', parseInt(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <FinancialInput field="channelMinimumFeeMloki" label="Opening Fee Floor" explanation="Minimum fee for opening a zero-conf channel." value={formData?.channelMinimumFeeMloki} source="MLOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <FinancialInput field="minChannelCapacityLoki" label="Floor Capacity" explanation="Absolute minimum size for any new channel." value={formData?.minChannelCapacityLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <FinancialInput field="additionalChannelCapacity" label="JIT Padding" explanation="Channel capacity added on top of the incoming payment when opening a new zero-conf channel." value={formData?.additionalChannelCapacity} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <FinancialInput field="jitMinChannelBalanceLoki" label="JIT Min Channel Balance" explanation="Minimum channel balance (capacity) in loki for JIT channels." value={formData?.jitMinChannelBalanceLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <Input label="Max Client-to-Self Delay" icon={Clock} explanation="Maximum ClientToSelfDelay for JIT channel opening fee params." type="number" value={formData?.jitMaxClientToSelfDelay} onChange={(v) => handleInputChange('jitMaxClientToSelfDelay', parseInt(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                      <Input label="Fee Params Validity (s)" icon={Activity} explanation="Validity duration in seconds for the JIT fee parameters." type="number" value={formData?.jitValidUntilSeconds} onChange={(v) => handleInputChange('jitValidUntilSeconds', parseInt(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    </div>
                </GlassCard>

                <GlassCard>
                  <SectionHeader title="LSPS1 Manual Orders" subtitle="Pro-active liquidity purchasing protocol" icon={Shield} />
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-x-12 gap-y-12">
                    <Input label="Required Channel Confirmations" icon={Clock} explanation="Minimum required channel confirmations for LSPS1 orders." type="number" value={formData?.lsps1MinRequiredChannelConfirmations} onChange={(v) => handleInputChange('lsps1MinRequiredChannelConfirmations', parseInt(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <Input label="Min Funding Confirms Within (blocks)" icon={Clock} explanation="Minimum funding confirms within blocks for LSPS1 orders." type="number" value={formData?.lsps1MinFundingConfirmsWithinBlocks} onChange={(v) => handleInputChange('lsps1MinFundingConfirmsWithinBlocks', parseInt(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <Input label="Max Expiry (Blocks)" icon={Scale} explanation="Maximum allowed channel lifespan in blocks." type="number" value={formData?.lsps1MaxChannelExpiryBlocks} onChange={(v) => handleInputChange('lsps1MaxChannelExpiryBlocks', parseInt(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <Input label="Order TTL (Seconds)" icon={Activity} explanation="Time before a pending order is cancelled." type="number" value={formData?.lsps1OrderExpirySeconds} onChange={(v) => handleInputChange('lsps1OrderExpirySeconds', parseInt(v))} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />

                    <div className="lg:col-span-3 pt-6 border-t border-outline-variant" />

                    <FinancialInput field="lsps1MinInitialClientBalanceLoki" label="Min Client Stake" explanation="Minimum balance client must fund initially." value={formData?.lsps1MinInitialClientBalanceLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <FinancialInput field="lsps1MaxInitialClientBalanceLoki" label="Max Client Stake" explanation="Maximum balance allowed for client stake." value={formData?.lsps1MaxInitialClientBalanceLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <FinancialInput field="lsps1MinInitialLspBalanceLoki" label="Min LSP Liquidity" explanation="Minimum capacity LSP must provide." value={formData?.lsps1MinInitialLspBalanceLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <FinancialInput field="lsps1MaxInitialLspBalanceLoki" label="Max LSP Liquidity" explanation="Maximum capacity LSP will provide." value={formData?.lsps1MaxInitialLspBalanceLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
                    <FinancialInput field="lsps1MinChannelBalanceLoki" label="Min Channel Capacity" explanation="Minimum channel balance (capacity) in loki for LSPS1 orders." value={formData?.lsps1MinChannelBalanceLoki} source="LOKI" onEdit={(f, s, v, l, e) => setEditingField({ field: f, source: s, value: v, label: l, explanation: e })} onInfo={(l, e) => setInfoModal({ label: l, explanation: e })} />
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
