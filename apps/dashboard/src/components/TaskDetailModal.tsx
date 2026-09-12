import React, { useEffect, useState } from 'react';
import { Task, VerificationResult } from '../types';
import { fetchTaskDetail, approveTask, rejectTask, createRunbook } from '../api';
import { 
  X, 
  CheckCircle2, 
  AlertTriangle, 
  ShieldCheck, 
  Terminal, 
  ChevronDown, 
  ChevronRight, 
  ShieldAlert,
  BookmarkPlus
} from 'lucide-react';

interface TaskDetailModalProps {
  task: Task;
  onClose: () => void;
  onTaskUpdated: () => void;
}

export const TaskDetailModal: React.FC<TaskDetailModalProps> = ({ task: initialTask, onClose, onTaskUpdated }) => {
  const [task, setTask] = useState<Task>(initialTask);
  const [verifications, setVerifications] = useState<VerificationResult[]>([]);
  const [expandedStepId, setExpandedStepId] = useState<string | null>(null);
  const [approvalNotes, setApprovalNotes] = useState('');
  const [isProcessingApproval, setIsProcessingApproval] = useState(false);
  const [runbookSaved, setRunbookSaved] = useState(false);

  const loadData = async () => {
    try {
      const data = await fetchTaskDetail(task.id);
      setTask(data.task);
      setVerifications(data.verifications || []);
    } catch (err) {
      console.error('Failed to load task details:', err);
    }
  };

  // Setup live SSE stream for real-time lifecycle events
  useEffect(() => {
    loadData();

    const eventSource = new EventSource(`/api/v1/tasks/${task.id}/events`);

    eventSource.addEventListener('STEP_OUTPUT', () => {
      loadData();
    });

    eventSource.addEventListener('STEP_COMPLETED', () => {
      loadData();
      onTaskUpdated();
    });

    eventSource.addEventListener('APPROVAL_REQUIRED', () => {
      loadData();
      onTaskUpdated();
    });

    eventSource.addEventListener('TASK_COMPLETED', () => {
      loadData();
      onTaskUpdated();
    });

    const interval = setInterval(loadData, 3000);

    return () => {
      eventSource.close();
      clearInterval(interval);
    };
  }, [task.id]);

  const handleApprove = async () => {
    setIsProcessingApproval(true);
    try {
      await approveTask(task.id, task.plan_version, approvalNotes);
      await loadData();
      onTaskUpdated();
    } catch (err: any) {
      alert(err.message || 'Approval failed');
    } finally {
      setIsProcessingApproval(false);
    }
  };

  const handleReject = async () => {
    setIsProcessingApproval(true);
    try {
      await rejectTask(task.id, task.plan_version, approvalNotes);
      await loadData();
      onTaskUpdated();
    } catch (err: any) {
      alert(err.message || 'Rejection failed');
    } finally {
      setIsProcessingApproval(false);
    }
  };

  const handleSaveAsRunbook = async () => {
    try {
      const slug = `runbook-${task.title.toLowerCase().replace(/[^a-z0-9]+/g, '-')}-${Date.now().toString().slice(-4)}`;
      await createRunbook({
        task_id: task.id,
        slug,
        title: task.title,
        description: `Generated from verified task: ${task.prompt}`,
      });
      setRunbookSaved(true);
    } catch (err: any) {
      alert(err.message || 'Failed to save runbook');
    }
  };

  // Progression steps mapping
  const stages = [
    { key: 'CREATED', label: 'Task Created' },
    { key: 'DISCOVERING', label: 'Host Discovery' },
    { key: 'PLANNING', label: 'AI Planning' },
    { key: 'WAITING_APPROVAL', label: 'Approval Gate' },
    { key: 'EXECUTING', label: 'Execution' },
    { key: 'VERIFYING', label: 'Verification' },
    { key: 'COMPLETED', label: 'Completed' },
  ];

  const getStageStatus = (stageKey: string) => {
    const order = ['CREATED', 'DISCOVERING', 'PLANNING', 'WAITING_APPROVAL', 'EXECUTING', 'VERIFYING', 'COMPLETED'];
    const currentIndex = order.indexOf(task.status);
    const stageIndex = order.indexOf(stageKey);

    if (task.status === 'FAILED') {
      return stageIndex === currentIndex ? 'failed' : stageIndex < currentIndex ? 'done' : 'pending';
    }
    if (task.status === 'CANCELLED') {
      return stageIndex === currentIndex ? 'cancelled' : stageIndex < currentIndex ? 'done' : 'pending';
    }
    if (stageIndex < currentIndex) return 'done';
    if (stageIndex === currentIndex) return 'current';
    return 'pending';
  };

  return (
    <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4 overflow-y-auto">
      <div className="bg-slate-900 border border-slate-800 rounded-xl w-full max-w-4xl max-h-[90vh] flex flex-col shadow-2xl overflow-hidden">
        {/* Header */}
        <div className="px-6 py-4 border-b border-slate-800 flex items-center justify-between bg-slate-950/40">
          <div className="flex items-center space-x-3">
            <div className="p-2 rounded-lg bg-blue-500/10 border border-blue-500/20 text-blue-400">
              <Terminal className="w-5 h-5" />
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <h3 className="text-sm font-bold text-white">{task.title}</h3>
                <span className={`text-[10px] font-mono px-2 py-0.5 rounded border font-semibold ${
                  task.status === 'COMPLETED' ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30' :
                  task.status === 'WAITING_APPROVAL' ? 'bg-amber-500/10 text-amber-300 border-amber-500/30 animate-pulse' :
                  task.status === 'FAILED' ? 'bg-red-500/10 text-red-400 border-red-500/30' :
                  'bg-blue-500/10 text-blue-400 border-blue-500/30'
                }`}>
                  {task.status}
                </span>
              </div>
              <p className="text-xs text-slate-400 font-mono mt-0.5">ID: {task.id}</p>
            </div>
          </div>

          <div className="flex items-center space-x-2">
            {task.status === 'COMPLETED' && (
              <button
                onClick={handleSaveAsRunbook}
                disabled={runbookSaved}
                className="flex items-center space-x-1.5 px-3 py-1.5 rounded text-xs font-semibold bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-400 border border-emerald-500/30 transition-colors"
              >
                <BookmarkPlus className="w-3.5 h-3.5" />
                <span>{runbookSaved ? 'Runbook Saved' : 'Save as Runbook'}</span>
              </button>
            )}
            <button
              onClick={onClose}
              className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
            >
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>

        {/* Content Body */}
        <div className="flex-1 p-6 overflow-y-auto space-y-6">
          {/* User Intent Box */}
          <div className="p-4 rounded-lg bg-slate-950 border border-slate-800 space-y-1">
            <span className="text-[10px] uppercase font-bold text-slate-500 tracking-wider">Natural Language Intent:</span>
            <p className="text-xs text-slate-200 font-mono bg-slate-900/50 p-2.5 rounded border border-slate-800/80">
              "{task.prompt}"
            </p>
          </div>

          {/* Lifecycle Timeline Bar */}
          <div className="p-4 rounded-lg bg-slate-950 border border-slate-800">
            <span className="text-[10px] uppercase font-bold text-slate-500 tracking-wider">State Machine Lifecycle:</span>
            <div className="mt-3 flex items-center justify-between relative">
              <div className="absolute left-0 top-1/2 -translate-y-1/2 w-full h-0.5 bg-slate-800 -z-0"></div>
              {stages.map((stage, idx) => {
                const status = getStageStatus(stage.key);
                return (
                  <div key={stage.key} className="flex flex-col items-center relative z-10">
                    <div className={`w-6 h-6 rounded-full flex items-center justify-center text-[10px] font-bold border ${
                      status === 'done' ? 'bg-emerald-500 text-slate-950 border-emerald-400' :
                      status === 'current' ? 'bg-blue-600 text-white border-blue-400 animate-pulse' :
                      status === 'failed' ? 'bg-red-500 text-white border-red-400' :
                      'bg-slate-900 text-slate-600 border-slate-800'
                    }`}>
                      {status === 'done' ? '✓' : idx + 1}
                    </div>
                    <span className={`text-[10px] mt-1 font-mono ${
                      status === 'current' ? 'text-blue-400 font-bold' :
                      status === 'done' ? 'text-emerald-400' :
                      'text-slate-500'
                    }`}>
                      {stage.label}
                    </span>
                  </div>
                );
              })}
            </div>
          </div>

          {/* APPROVAL GATE CARD (if WAITING_APPROVAL) */}
          {task.status === 'WAITING_APPROVAL' && (
            <div className="p-5 rounded-lg bg-amber-950/30 border-2 border-amber-500/40 space-y-4">
              <div className="flex items-center space-x-3">
                <ShieldAlert className="w-6 h-6 text-amber-400" />
                <div>
                  <h4 className="text-sm font-bold text-amber-200">Operator Approval Gate (Plan v{task.plan_version})</h4>
                  <p className="text-xs text-amber-300/80">
                    Overall Risk Classification: <strong>{task.risk_level}</strong>. Please review the planned operations below.
                  </p>
                </div>
              </div>

              <div className="space-y-2">
                <input
                  type="text"
                  placeholder="Optional approval notes (e.g. Approved for maintenance window)"
                  value={approvalNotes}
                  onChange={(e) => setApprovalNotes(e.target.value)}
                  className="w-full px-3 py-2 text-xs bg-slate-900 border border-slate-700 rounded-md text-slate-200 focus:outline-none focus:border-amber-500"
                />
                <div className="flex space-x-3 pt-1">
                  <button
                    onClick={handleApprove}
                    disabled={isProcessingApproval}
                    className="flex-1 py-2 px-4 rounded-md text-xs font-bold bg-emerald-600 hover:bg-emerald-500 text-white transition-colors"
                  >
                    {isProcessingApproval ? 'Processing...' : 'Approve Execution Plan'}
                  </button>
                  <button
                    onClick={handleReject}
                    disabled={isProcessingApproval}
                    className="py-2 px-4 rounded-md text-xs font-bold bg-red-600/20 hover:bg-red-600/30 text-red-300 border border-red-500/40 transition-colors"
                  >
                    Reject Plan
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Execution Steps List */}
          <div className="space-y-3">
            <h4 className="text-xs font-bold uppercase tracking-wider text-slate-400 flex items-center justify-between">
              <span>Planned Steps ({task.steps ? task.steps.length : 0})</span>
              <span className="text-[10px] text-slate-500 lowercase font-normal">click row to view stdout/stderr</span>
            </h4>

            {task.steps && task.steps.length > 0 ? (
              <div className="space-y-2">
                {task.steps.map((step) => {
                  const isExpanded = expandedStepId === step.id;
                  return (
                    <div
                      key={step.id}
                      className="rounded-lg bg-slate-950 border border-slate-800 overflow-hidden text-xs"
                    >
                      <div
                        onClick={() => setExpandedStepId(isExpanded ? null : step.id)}
                        className="p-3 flex items-center justify-between cursor-pointer hover:bg-slate-900/50 transition-colors"
                      >
                        <div className="flex items-center space-x-3">
                          {isExpanded ? <ChevronDown className="w-4 h-4 text-slate-400" /> : <ChevronRight className="w-4 h-4 text-slate-400" />}
                          <span className="font-mono text-slate-500 font-bold">#{step.step_order}</span>
                          <span className="font-mono font-semibold text-slate-200">{step.action}</span>
                          <span className={`px-1.5 py-0.2 text-[9px] rounded font-semibold border ${
                            step.risk_level === 'READ_ONLY' ? 'bg-slate-800 text-slate-400 border-slate-700' :
                            step.risk_level === 'MEDIUM' ? 'bg-amber-500/10 text-amber-400 border-amber-500/30' :
                            'bg-red-500/10 text-red-400 border-red-500/30'
                          }`}>
                            {step.risk_level}
                          </span>
                        </div>

                        <div className="flex items-center space-x-3">
                          <span className={`px-2 py-0.5 rounded text-[10px] font-mono font-bold uppercase ${
                            step.status === 'SUCCESS' ? 'text-emerald-400 bg-emerald-500/10' :
                            step.status === 'RUNNING' ? 'text-blue-400 bg-blue-500/10 animate-pulse' :
                            step.status === 'FAILED' ? 'text-red-400 bg-red-500/10' :
                            'text-slate-500 bg-slate-900'
                          }`}>
                            {step.status}
                          </span>
                        </div>
                      </div>

                      {/* Expandable Step Terminal Logs & Arguments */}
                      {isExpanded && (
                        <div className="p-3 bg-slate-900/90 border-t border-slate-800/80 space-y-2 font-mono text-[11px]">
                          <div>
                            <span className="text-slate-500 uppercase text-[9px]">Arguments:</span>
                            <pre className="p-2 rounded bg-slate-950 text-slate-300 overflow-x-auto mt-1">
                              {JSON.stringify(step.arguments, null, 2)}
                            </pre>
                          </div>
                          {step.stdout && (
                            <div>
                              <span className="text-emerald-400 uppercase text-[9px]">STDOUT:</span>
                              <pre className="p-2 rounded bg-slate-950 text-emerald-300/90 overflow-x-auto mt-1 max-h-48">
                                {step.stdout}
                              </pre>
                            </div>
                          )}
                          {step.stderr && (
                            <div>
                              <span className="text-red-400 uppercase text-[9px]">STDERR:</span>
                              <pre className="p-2 rounded bg-slate-950 text-red-300 overflow-x-auto mt-1 max-h-48">
                                {step.stderr}
                              </pre>
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="text-center py-6 text-slate-500 text-xs font-mono">
                Generating plan...
              </div>
            )}
          </div>

          {/* Deterministic Verification Suite Results */}
          {verifications.length > 0 && (
            <div className="p-4 rounded-lg bg-slate-950 border border-slate-800 space-y-3">
              <div className="flex items-center space-x-2">
                <ShieldCheck className="w-4 h-4 text-emerald-400" />
                <h4 className="text-xs font-bold text-white uppercase tracking-wider">
                  Independent Deterministic Verification
                </h4>
              </div>
              <div className="space-y-2">
                {verifications.map((v, i) => (
                  <div key={i} className="p-2.5 rounded bg-slate-900 border border-slate-800 flex items-center justify-between text-xs">
                    <div className="flex items-center space-x-2.5 font-mono">
                      {v.passed ? (
                        <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                      ) : (
                        <AlertTriangle className="w-4 h-4 text-red-400" />
                      )}
                      <div>
                        <span className="text-slate-300 font-semibold">{v.check_type}: </span>
                        <span className="text-blue-400">{v.target}</span>
                      </div>
                    </div>
                    <span className={`text-[10px] font-bold uppercase px-2 py-0.5 rounded ${
                      v.passed ? 'bg-emerald-500/10 text-emerald-400' : 'bg-red-500/10 text-red-400'
                    }`}>
                      {v.passed ? 'PASSED' : 'FAILED'}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
