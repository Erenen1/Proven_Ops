import React from 'react';
import { Task } from '../types';
import { ShieldAlert, ArrowUpRight } from 'lucide-react';

interface ApprovalsViewProps {
  tasks: Task[];
  onSelectTask: (task: Task) => void;
}

export const ApprovalsView: React.FC<ApprovalsViewProps> = ({ tasks, onSelectTask }) => {
  const pendingApprovals = tasks.filter((t) => t.status === 'WAITING_APPROVAL');

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-bold text-white tracking-tight">Pending Approvals Inbox</h2>
        <p className="text-xs text-slate-400">
          State-changing operations requiring human sign-off before server execution
        </p>
      </div>

      {pendingApprovals.length === 0 ? (
        <div className="p-12 text-center rounded-lg bg-slate-900 border border-slate-800">
          <ShieldAlert className="w-10 h-10 text-emerald-500/40 mx-auto mb-2" />
          <h3 className="text-sm font-semibold text-slate-300">All Clear</h3>
          <p className="text-xs text-slate-500 mt-1">There are no pending operations awaiting operator approval.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4">
          {pendingApprovals.map((task) => (
            <div
              key={task.id}
              onClick={() => onSelectTask(task)}
              className="p-5 rounded-lg bg-slate-900 border border-amber-500/40 hover:border-amber-500/70 transition-colors cursor-pointer space-y-3"
            >
              <div className="flex items-start justify-between">
                <div>
                  <div className="flex items-center space-x-2">
                    <span className="px-2 py-0.5 text-[10px] font-bold rounded bg-amber-500/20 text-amber-300 border border-amber-500/40 uppercase">
                      Risk: {task.risk_level}
                    </span>
                    <span className="text-xs font-mono text-slate-400">Plan v{task.plan_version}</span>
                  </div>
                  <h4 className="text-sm font-bold text-white mt-1.5">{task.title}</h4>
                  <p className="text-xs text-slate-400 font-mono mt-0.5">{task.prompt}</p>
                </div>

                <button className="px-3 py-1.5 rounded text-xs font-bold bg-amber-500 hover:bg-amber-400 text-slate-950 flex items-center space-x-1">
                  <span>Review Gate</span>
                  <ArrowUpRight className="w-3.5 h-3.5" />
                </button>
              </div>

              {/* Step Summary Badges */}
              {task.steps && (
                <div className="pt-2 border-t border-slate-800 flex flex-wrap gap-2 text-xs font-mono">
                  {task.steps.map((step) => (
                    <span
                      key={step.id}
                      className="px-2 py-0.5 rounded bg-slate-950 border border-slate-800 text-[11px] text-slate-300"
                    >
                      {step.step_order}. {step.action} ({step.risk_level})
                    </span>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
