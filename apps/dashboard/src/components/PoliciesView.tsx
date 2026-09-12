import React from 'react';
import { ShieldCheck, Lock, AlertOctagon } from 'lucide-react';

export const PoliciesView: React.FC = () => {
  const policies = [
    {
      name: 'Allow Read-Only Operations Globally',
      scope: 'Global',
      pattern: 'get_*, check_*, read_*, http_probe, docker_ps',
      risk: 'READ_ONLY',
      approval: 'Automatic',
      description: 'Host discovery and telemetry tools run without requiring operator approval.',
    },
    {
      name: 'Production Package Install Guard',
      scope: 'Environment (production)',
      pattern: 'install_package',
      risk: 'MEDIUM',
      approval: 'Required',
      description: 'Apt package installations modify host binary state and require explicit sign-off.',
    },
    {
      name: 'Systemd Service State Modification',
      scope: 'Global',
      pattern: 'start_service, stop_service, restart_service',
      risk: 'MEDIUM',
      approval: 'Required',
      description: 'Starting or restarting system services requires operator validation to prevent downtime.',
    },
    {
      name: 'High Risk Raw Command Guardrail',
      scope: 'Global',
      pattern: 'execute_command',
      risk: 'HIGH',
      approval: 'Explicit',
      description: 'Guarded raw command fallback subjected to AST tokenizer and strict denylists.',
    },
  ];

  const forbiddenRules = [
    "Indiscriminate recursive deletions (rm -rf / or rm -rf /*)",
    "Block device formatting and low-level disk tools (mkfs, fdisk, parted, dd)",
    "Host state termination (shutdown, reboot, poweroff, halt, init 0/6)",
    "User account & credential manipulation (userdel, groupdel, passwd -d)",
    "Indiscriminate firewall flushes (iptables -F, nft flush ruleset)",
    "Remote script pipe execution (curl http://... | bash, wget | sh)",
    "Resource exhaustion attacks & fork bombs (:(){ :|:& };:)",
  ];

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-bold text-white tracking-tight">Security & Policy Engine</h2>
        <p className="text-xs text-slate-400">
          Enforced boundaries separating untrusted LLM reasoning from host execution
        </p>
      </div>

      {/* Baseline Policies Table */}
      <div className="rounded-lg bg-slate-900 border border-slate-800 overflow-hidden space-y-2 p-5">
        <h3 className="text-sm font-bold text-white flex items-center space-x-2">
          <ShieldCheck className="w-4 h-4 text-blue-400" />
          <span>Active Policy Set</span>
        </h3>
        <p className="text-xs text-slate-400">
          Rules governing whether proposed steps execute automatically or require operator sign-off.
        </p>

        <div className="mt-4 grid grid-cols-1 md:grid-cols-2 gap-4">
          {policies.map((p, idx) => (
            <div key={idx} className="p-4 rounded-lg bg-slate-950 border border-slate-800 space-y-2 text-xs">
              <div className="flex items-start justify-between">
                <h4 className="font-bold text-slate-200">{p.name}</h4>
                <span className={`px-2 py-0.5 rounded text-[10px] font-mono font-bold uppercase border ${
                  p.risk === 'READ_ONLY' ? 'bg-slate-800 text-slate-300 border-slate-700' :
                  p.risk === 'MEDIUM' ? 'bg-amber-500/10 text-amber-300 border-amber-500/30' :
                  'bg-red-500/10 text-red-400 border-red-500/30'
                }`}>
                  {p.risk}
                </span>
              </div>
              <p className="text-slate-400 text-[11px]">{p.description}</p>
              <div className="pt-2 border-t border-slate-900 font-mono text-[10px] text-slate-500 flex justify-between">
                <span>Pattern: <strong className="text-slate-300">{p.pattern}</strong></span>
                <span>Scope: <strong className="text-slate-300">{p.scope}</strong></span>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Forbidden Commands Callout */}
      <div className="p-5 rounded-lg bg-red-950/20 border border-red-500/30 space-y-3">
        <div className="flex items-center space-x-2">
          <AlertOctagon className="w-5 h-5 text-red-400" />
          <h3 className="text-sm font-bold text-red-300">Strictly Forbidden Destructive Operations</h3>
        </div>
        <p className="text-xs text-red-300/80">
          The following operations are rejected instantaneously at both Control Plane and Agent AST guard levels, even if proposed by LLM:
        </p>
        <ul className="grid grid-cols-1 md:grid-cols-2 gap-2 text-xs font-mono text-slate-300 pt-1">
          {forbiddenRules.map((rule, idx) => (
            <li key={idx} className="flex items-center space-x-2 p-2 rounded bg-slate-950/60 border border-slate-800/80">
              <Lock className="w-3.5 h-3.5 text-red-400 shrink-0" />
              <span>{rule}</span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
};
