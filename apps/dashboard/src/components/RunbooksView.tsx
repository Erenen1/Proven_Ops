import React, { useState } from 'react';
import { Runbook, Agent, Task, RunbookExecutionResult } from '../types';
import { dryRunRunbook, executeRunbook } from '../api';
import {
  BookOpen,
  Play,
  ShieldCheck,
  ShieldAlert,
  Terminal,
  RefreshCw,
  X,
  Server,
  Sliders,
  CheckCircle2,
  AlertTriangle,
} from 'lucide-react';

interface RunbooksViewProps {
  runbooks: Runbook[];
  agents?: Agent[];
  onTaskCreated?: (task: Task) => void;
}

export const RunbooksView: React.FC<RunbooksViewProps> = ({
  runbooks,
  agents = [],
  onTaskCreated,
}) => {
  const [activeRunbook, setActiveRunbook] = useState<Runbook | null>(null);
  const [paramValues, setParamValues] = useState<Record<string, string>>({});
  const [selectedAgentId, setSelectedAgentId] = useState<string>('');
  const [dryRunResult, setDryRunResult] = useState<RunbookExecutionResult | null>(null);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [executing, setExecuting] = useState<boolean>(false);
  const [errorMsg, setErrorMsg] = useState<string>('');

  const openRunModal = (rb: Runbook) => {
    setActiveRunbook(rb);
    setErrorMsg('');
    setDryRunResult(null);

    // Populate initial parameter defaults
    const initialParams: Record<string, string> = {};
    if (rb.variables) {
      rb.variables.forEach((v) => {
        initialParams[v.name] = v.default || '';
      });
    }
    setParamValues(initialParams);

    // Default to first agent if available
    if (agents.length > 0) {
      setSelectedAgentId(agents[0].id);
    }
  };

  const closeModal = () => {
    setActiveRunbook(null);
    setDryRunResult(null);
    setErrorMsg('');
  };

  const handleSimulate = async () => {
    if (!activeRunbook) return;
    setIsLoading(true);
    setErrorMsg('');
    try {
      const res = await dryRunRunbook(activeRunbook.id, paramValues);
      setDryRunResult(res);
    } catch (err: any) {
      setErrorMsg(err.message || 'Simulation failed');
    } finally {
      setIsLoading(false);
    }
  };

  const handleExecute = async () => {
    if (!activeRunbook) return;
    setExecuting(true);
    setErrorMsg('');
    try {
      const task = await executeRunbook(activeRunbook.id, {
        target_agent_ids: selectedAgentId ? [selectedAgentId] : [],
        parameters: paramValues,
        dry_run: false,
      });
      if (onTaskCreated) {
        onTaskCreated(task);
      }
      closeModal();
    } catch (err: any) {
      setErrorMsg(err.message || 'Runbook execution failed');
    } finally {
      setExecuting(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-white tracking-tight">Enterprise SRE Runbooks</h2>
          <p className="text-xs text-slate-400">
            Pre-verified deterministic operations with parameter substitution and pre-execution policy dry-run.
          </p>
        </div>
        <span className="text-xs font-mono text-slate-400">
          Available Runbooks: <strong className="text-white">{runbooks.length}</strong>
        </span>
      </div>

      {runbooks.length === 0 ? (
        <div className="p-12 text-center rounded-xl bg-slate-900/60 border border-slate-800">
          <BookOpen className="w-10 h-10 text-slate-600 mx-auto mb-2" />
          <h3 className="text-sm font-semibold text-slate-300">No Runbooks Registered</h3>
          <p className="text-xs text-slate-500 mt-1">
            Standard SRE runbooks will seed automatically upon refreshing.
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {runbooks.map((rb) => (
            <div
              key={rb.id}
              className="p-5 rounded-xl bg-slate-900/80 border border-slate-800/80 hover:border-slate-700 transition-all flex flex-col justify-between space-y-4"
            >
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <span className="font-mono text-[11px] text-blue-400 font-bold bg-blue-950/60 px-2 py-0.5 rounded border border-blue-900/40">
                    {rb.slug}
                  </span>
                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-slate-800 text-slate-400 border border-slate-700">
                    v{rb.latest_version}
                  </span>
                </div>
                <h4 className="text-sm font-bold text-white leading-snug">{rb.title}</h4>
                <p className="text-xs text-slate-400 line-clamp-2">{rb.description}</p>
              </div>

              {rb.variables && rb.variables.length > 0 && (
                <div className="space-y-1">
                  <span className="text-[10px] uppercase tracking-wider text-slate-500 font-mono">Parameters:</span>
                  <div className="flex flex-wrap gap-1">
                    {rb.variables.map((v) => (
                      <span
                        key={v.name}
                        className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-800/80 text-emerald-400 border border-emerald-950/60"
                      >
                        {`{{${v.name}}}`}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              <div className="pt-3 border-t border-slate-800/80 flex items-center justify-between">
                <span className="text-[11px] text-slate-500 font-mono">
                  {rb.steps ? `${rb.steps.length} verified steps` : 'Deterministic'}
                </span>
                <button
                  onClick={() => openRunModal(rb)}
                  className="px-3 py-1.5 rounded-lg text-xs font-semibold bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500 text-white flex items-center space-x-1.5 transition-all shadow-sm shadow-blue-500/10"
                >
                  <Play className="w-3 h-3" />
                  <span>Configure & Run</span>
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Runbook Drawer / Execution Modal */}
      {activeRunbook && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm">
          <div className="w-full max-w-2xl bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
            {/* Header */}
            <div className="px-6 py-4 border-b border-slate-800 flex items-center justify-between bg-slate-950/40">
              <div className="flex items-center space-x-2.5">
                <Sliders className="w-5 h-5 text-blue-400" />
                <div>
                  <h3 className="text-base font-bold text-white">{activeRunbook.title}</h3>
                  <span className="text-xs font-mono text-slate-400">{activeRunbook.slug} (v{activeRunbook.latest_version})</span>
                </div>
              </div>
              <button
                onClick={closeModal}
                className="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* Body */}
            <div className="p-6 overflow-y-auto space-y-6 flex-1 text-xs">
              {/* Description */}
              <div className="p-3.5 rounded-lg bg-slate-950/60 border border-slate-800/80 text-slate-300">
                {activeRunbook.description}
              </div>

              {/* Target Agent Selector */}
              <div className="space-y-2">
                <label className="text-slate-300 font-semibold flex items-center space-x-1.5">
                  <Server className="w-4 h-4 text-slate-400" />
                  <span>Target Fleet Node</span>
                </label>
                <select
                  value={selectedAgentId}
                  onChange={(e) => setSelectedAgentId(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white font-mono text-xs focus:outline-none focus:border-blue-500"
                >
                  {agents.length === 0 ? (
                    <option value="">No registered agents available (will simulate only)</option>
                  ) : (
                    agents.map((ag) => (
                      <option key={ag.id} value={ag.id}>
                        {ag.hostname} ({ag.ip_address}) - {ag.distribution} {ag.version}
                      </option>
                    ))
                  )}
                </select>
              </div>

              {/* Parameters */}
              {activeRunbook.variables && activeRunbook.variables.length > 0 && (
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <span className="font-semibold text-slate-200">Runbook Parameters</span>
                    <span className="text-[10px] text-slate-500 font-mono">Substituted as &#123;&#123;param&#125;&#125;</span>
                  </div>

                  <div className="space-y-2.5">
                    {activeRunbook.variables.map((v) => (
                      <div key={v.name} className="space-y-1">
                        <div className="flex items-center justify-between">
                          <label className="font-mono text-slate-300">
                            {v.name} {v.required && <span className="text-rose-400">*</span>}
                          </label>
                          <span className="text-[11px] text-slate-500">{v.description}</span>
                        </div>
                        <input
                          type="text"
                          value={paramValues[v.name] || ''}
                          onChange={(e) =>
                            setParamValues({ ...paramValues, [v.name]: e.target.value })
                          }
                          placeholder={v.default ? `Default: ${v.default}` : 'Enter value...'}
                          className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white font-mono text-xs focus:outline-none focus:border-blue-500"
                        />
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Dry-Run Simulation Result Banner */}
              {dryRunResult && (
                <div
                  className={`p-4 rounded-xl border space-y-3 ${
                    dryRunResult.policy_pass
                      ? 'bg-emerald-950/20 border-emerald-800/40 text-emerald-300'
                      : 'bg-rose-950/20 border-rose-800/40 text-rose-300'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-2">
                      {dryRunResult.policy_pass ? (
                        <ShieldCheck className="w-5 h-5 text-emerald-400" />
                      ) : (
                        <ShieldAlert className="w-5 h-5 text-rose-400" />
                      )}
                      <span className="font-bold text-sm">
                        {dryRunResult.policy_pass ? 'Policy Simulation Passed' : 'Policy Violation Detected'}
                      </span>
                    </div>
                    <span className="font-mono text-[11px] px-2 py-0.5 rounded bg-slate-900 border border-slate-800 font-bold">
                      Max Risk: {dryRunResult.max_risk}
                    </span>
                  </div>

                  {dryRunResult.validations && (
                    <div className="space-y-1 font-mono text-[11px] pt-1">
                      {dryRunResult.validations.map((v, i) => (
                        <div key={i} className="flex items-center space-x-1.5">
                          {dryRunResult.policy_pass ? (
                            <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400 shrink-0" />
                          ) : (
                            <AlertTriangle className="w-3.5 h-3.5 text-rose-400 shrink-0" />
                          )}
                          <span>{v}</span>
                        </div>
                      ))}
                    </div>
                  )}

                  {dryRunResult.plan && (
                    <div className="mt-2 pt-2 border-t border-slate-800/60 font-mono text-[11px] space-y-1 text-slate-300">
                      <span className="text-slate-400 font-sans font-semibold">Planned Steps ({dryRunResult.plan.steps.length}):</span>
                      {dryRunResult.plan.steps.map((st, i) => (
                        <div key={i} className="p-2 rounded bg-slate-950/80 border border-slate-800 flex items-start space-x-2">
                          <Terminal className="w-3.5 h-3.5 text-blue-400 mt-0.5 shrink-0" />
                          <div>
                            <span className="font-bold text-white">{st.action}</span>
                            <span className="text-slate-400 ml-2">{st.reason}</span>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {errorMsg && (
                <div className="p-3 rounded-lg bg-rose-950/30 border border-rose-800 text-rose-300 flex items-center space-x-2">
                  <AlertTriangle className="w-4 h-4 shrink-0" />
                  <span>{errorMsg}</span>
                </div>
              )}
            </div>

            {/* Footer */}
            <div className="px-6 py-4 border-t border-slate-800 bg-slate-950/60 flex items-center justify-between">
              <button
                type="button"
                onClick={handleSimulate}
                disabled={isLoading}
                className="px-4 py-2 rounded-lg text-xs font-semibold bg-slate-800 hover:bg-slate-700 text-slate-200 flex items-center space-x-1.5 transition-colors border border-slate-700"
              >
                {isLoading ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : <ShieldCheck className="w-3.5 h-3.5" />}
                <span>Simulate / Dry-Run</span>
              </button>

              <div className="flex items-center space-x-3">
                <button
                  type="button"
                  onClick={closeModal}
                  className="px-4 py-2 rounded-lg text-xs font-semibold text-slate-400 hover:text-white transition-colors"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={handleExecute}
                  disabled={executing}
                  className="px-4 py-2 rounded-lg text-xs font-semibold bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white flex items-center space-x-1.5 transition-all shadow-sm shadow-emerald-500/20"
                >
                  {executing ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : <Play className="w-3.5 h-3.5" />}
                  <span>Execute on Fleet</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
