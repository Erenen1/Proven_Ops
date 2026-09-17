import React from 'react';
import { Agent, Task } from '../types';
import { Server, Terminal, ShieldAlert, CheckCircle2, Clock, ArrowRight, ShieldCheck } from 'lucide-react';

interface OverviewViewProps {
  agents: Agent[];
  tasks: Task[];
  onSelectTask: (task: Task) => void;
  onNavigateTab: (tab: string) => void;
  onNewTaskClick: () => void;
}

export const OverviewView: React.FC<OverviewViewProps> = ({
  agents,
  tasks,
  onSelectTask,
  onNavigateTab,
  onNewTaskClick,
}) => {
  const onlineAgents = agents.filter((a) => a.status === 'online');
  const runningTasks = tasks.filter((t) => ['DISCOVERING', 'PLANNING', 'EXECUTING', 'VERIFYING'].includes(t.status));
  const pendingApprovals = tasks.filter((t) => t.status === 'WAITING_APPROVAL');
  const completedTasks = tasks.filter((t) => t.status === 'COMPLETED');

  const recentTasks = [...tasks].sort(
    (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  ).slice(0, 5);

  return (
    <div className="space-y-6">
      {/* Pending Approval Banner */}
      {pendingApprovals.length > 0 && (
        <div className="p-4 rounded-lg bg-amber-950/40 border border-amber-500/40 flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <ShieldAlert className="w-5 h-5 text-amber-400 shrink-0" />
            <div>
              <h4 className="text-sm font-semibold text-amber-200">
                Action Required: {pendingApprovals.length} task(s) awaiting operator approval
              </h4>
              <p className="text-xs text-amber-400/80">
                ProvenOps policy engine classified changes as Medium/High risk. Review steps before execution.
              </p>
            </div>
          </div>
          <button
            onClick={() => onNavigateTab('approvals')}
            className="px-3 py-1.5 rounded text-xs font-semibold bg-amber-500 hover:bg-amber-400 text-slate-950 flex items-center space-x-1.5 transition-colors"
          >
            <span>Review Approvals</span>
            <ArrowRight className="w-3.5 h-3.5" />
          </button>
        </div>
      )}

      {/* Metrics Row */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="p-4 rounded-lg bg-slate-900 border border-slate-800">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-slate-400">Connected Fleet</span>
            <Server className="w-4 h-4 text-blue-400" />
          </div>
          <div className="mt-2 flex items-baseline space-x-2">
            <span className="text-2xl font-bold font-mono text-white">{onlineAgents.length}</span>
            <span className="text-xs text-slate-500">/ {agents.length} nodes</span>
          </div>
          <p className="mt-1 text-[11px] text-emerald-400 flex items-center space-x-1">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
            <span>mTLS Outbound Active</span>
          </p>
        </div>

        <div className="p-4 rounded-lg bg-slate-900 border border-slate-800">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-slate-400">Active Tasks</span>
            <Clock className="w-4 h-4 text-blue-400" />
          </div>
          <div className="mt-2 flex items-baseline space-x-2">
            <span className="text-2xl font-bold font-mono text-white">{runningTasks.length}</span>
            <span className="text-xs text-slate-500">running</span>
          </div>
          <p className="mt-1 text-[11px] text-slate-400">Automated state machine tracking</p>
        </div>

        <div className="p-4 rounded-lg bg-slate-900 border border-slate-800">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-slate-400">Pending Approvals</span>
            <ShieldCheck className="w-4 h-4 text-amber-400" />
          </div>
          <div className="mt-2 flex items-baseline space-x-2">
            <span className="text-2xl font-bold font-mono text-amber-300">{pendingApprovals.length}</span>
            <span className="text-xs text-slate-500">gates held</span>
          </div>
          <p className="mt-1 text-[11px] text-slate-400">Zero unapproved changes</p>
        </div>

        <div className="p-4 rounded-lg bg-slate-900 border border-slate-800">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-slate-400">Verified Completed</span>
            <CheckCircle2 className="w-4 h-4 text-emerald-400" />
          </div>
          <div className="mt-2 flex items-baseline space-x-2">
            <span className="text-2xl font-bold font-mono text-emerald-400">{completedTasks.length}</span>
            <span className="text-xs text-slate-500">operations</span>
          </div>
          <p className="mt-1 text-[11px] text-emerald-400/80">100% Deterministic verified</p>
        </div>
      </div>

      {/* Main Grid: Recent Operations & Fleet Quick Glance */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Recent Tasks List */}
        <div className="lg:col-span-2 p-5 rounded-lg bg-slate-900 border border-slate-800">
          <div className="flex items-center justify-between pb-4 border-b border-slate-800">
            <div>
              <h3 className="text-sm font-semibold text-white">Recent Operations</h3>
              <p className="text-xs text-slate-400">Infrastructure tasks managed by ProvenOps</p>
            </div>
            <button
              onClick={() => onNavigateTab('tasks')}
              className="text-xs text-blue-400 hover:text-blue-300 flex items-center space-x-1"
            >
              <span>View all</span>
              <ArrowRight className="w-3.5 h-3.5" />
            </button>
          </div>

          <div className="mt-4 divide-y divide-slate-800">
            {recentTasks.length === 0 ? (
              <div className="text-center py-8 text-slate-500 text-xs">
                No tasks executed yet. Click "New Task" to begin your first operation.
              </div>
            ) : (
              recentTasks.map((task) => (
                <div
                  key={task.id}
                  onClick={() => onSelectTask(task)}
                  className="py-3 flex items-center justify-between hover:bg-slate-800/40 px-2 rounded-md cursor-pointer transition-colors"
                >
                  <div className="flex items-start space-x-3">
                    <div className="mt-0.5">
                      <Terminal className="w-4 h-4 text-slate-400" />
                    </div>
                    <div>
                      <h4 className="text-xs font-semibold text-slate-200">{task.title}</h4>
                      <p className="text-[11px] text-slate-500 font-mono line-clamp-1">{task.prompt}</p>
                    </div>
                  </div>

                  <div className="flex items-center space-x-3">
                    <span className={`text-[10px] uppercase font-bold px-2 py-0.5 rounded border ${
                      task.status === 'COMPLETED' ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30' :
                      task.status === 'WAITING_APPROVAL' ? 'bg-amber-500/10 text-amber-300 border-amber-500/30 animate-pulse' :
                      task.status === 'FAILED' ? 'bg-red-500/10 text-red-400 border-red-500/30' :
                      'bg-blue-500/10 text-blue-400 border-blue-500/30'
                    }`}>
                      {task.status}
                    </span>
                    <span className="text-[11px] text-slate-500">
                      {new Date(task.created_at).toLocaleTimeString()}
                    </span>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>

        {/* Quick Launch & Fleet summary */}
        <div className="space-y-4">
          <div className="p-5 rounded-lg bg-slate-900 border border-slate-800">
            <h3 className="text-sm font-semibold text-white">Quick Infrastructure Intent</h3>
            <p className="text-xs text-slate-400 mt-1">Prompt the AI planner with standard operations:</p>
            
            <div className="mt-4 space-y-2">
              {[
                "Install nginx on server X and expose it on port 8080",
                "Install Docker and verify daemon status",
                "Diagnose why a systemd service is failing",
                "Inspect open listening ports and CPU usage",
              ].map((template, idx) => (
                <button
                  key={idx}
                  onClick={onNewTaskClick}
                  className="w-full text-left p-2.5 rounded bg-slate-850 hover:bg-slate-800 border border-slate-800 text-xs text-slate-300 hover:text-white transition-colors"
                >
                  <p className="font-mono text-[11px] text-blue-400">intent:</p>
                  <p className="line-clamp-1 mt-0.5">{template}</p>
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
