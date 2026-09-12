import React, { useState } from 'react';
import { Task } from '../types';
import { Terminal, ShieldAlert, CheckCircle2, XCircle, ArrowUpRight } from 'lucide-react';

interface TasksViewProps {
  tasks: Task[];
  onSelectTask: (task: Task) => void;
  onNewTaskClick?: () => void;
}

export const TasksView: React.FC<TasksViewProps> = ({ tasks, onSelectTask }) => {
  const [filter, setFilter] = useState<'ALL' | 'ACTIVE' | 'APPROVAL' | 'COMPLETED' | 'FAILED'>('ALL');

  const filteredTasks = tasks.filter((t) => {
    if (filter === 'ACTIVE') return ['DISCOVERING', 'PLANNING', 'EXECUTING', 'VERIFYING'].includes(t.status);
    if (filter === 'APPROVAL') return t.status === 'WAITING_APPROVAL';
    if (filter === 'COMPLETED') return t.status === 'COMPLETED';
    if (filter === 'FAILED') return t.status === 'FAILED';
    return true;
  });

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h2 className="text-lg font-bold text-white tracking-tight">Operations Log</h2>
          <p className="text-xs text-slate-400">
            Natural-language infrastructure tasks planned, policy-checked, and executed
          </p>
        </div>

        {/* Filter Pills */}
        <div className="flex items-center space-x-1 bg-slate-900 p-1 rounded-lg border border-slate-800 text-xs">
          {[
            { id: 'ALL', label: 'All Tasks' },
            { id: 'ACTIVE', label: 'Active' },
            { id: 'APPROVAL', label: 'Approvals' },
            { id: 'COMPLETED', label: 'Completed' },
            { id: 'FAILED', label: 'Failed' },
          ].map((tab) => (
            <button
              key={tab.id}
              onClick={() => setFilter(tab.id as any)}
              className={`px-3 py-1 rounded-md transition-colors font-medium ${
                filter === tab.id
                  ? 'bg-slate-800 text-white font-semibold'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      {filteredTasks.length === 0 ? (
        <div className="p-12 text-center rounded-lg bg-slate-900 border border-slate-800">
          <Terminal className="w-10 h-10 text-slate-600 mx-auto mb-2" />
          <h3 className="text-sm font-semibold text-slate-300">No tasks found</h3>
          <p className="text-xs text-slate-500 mt-1">There are no tasks matching the selected filter.</p>
        </div>
      ) : (
        <div className="rounded-lg bg-slate-900 border border-slate-800 overflow-hidden">
          <table className="w-full text-left text-xs">
            <thead className="bg-slate-950/60 border-b border-slate-800 text-slate-400 font-mono uppercase text-[10px]">
              <tr>
                <th className="py-3 px-4">Task & Intent</th>
                <th className="py-3 px-4">Target Agent</th>
                <th className="py-3 px-4">State</th>
                <th className="py-3 px-4">Risk Level</th>
                <th className="py-3 px-4">Created</th>
                <th className="py-3 px-4 text-right">Action</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800">
              {filteredTasks.map((task) => (
                <tr
                  key={task.id}
                  onClick={() => onSelectTask(task)}
                  className="hover:bg-slate-800/40 cursor-pointer transition-colors"
                >
                  <td className="py-3 px-4">
                    <div className="font-semibold text-slate-200">{task.title}</div>
                    <div className="text-slate-500 font-mono text-[11px] line-clamp-1 mt-0.5">
                      {task.prompt}
                    </div>
                  </td>
                  <td className="py-3 px-4 font-mono text-slate-300">
                    {task.target_agent_ids && task.target_agent_ids.length > 0 ? task.target_agent_ids[0] : 'N/A'}
                  </td>
                  <td className="py-3 px-4">
                    <span className={`inline-flex items-center space-x-1 px-2 py-0.5 rounded text-[10px] font-bold uppercase border ${
                      task.status === 'COMPLETED' ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30' :
                      task.status === 'WAITING_APPROVAL' ? 'bg-amber-500/10 text-amber-300 border-amber-500/30 animate-pulse' :
                      task.status === 'FAILED' ? 'bg-red-500/10 text-red-400 border-red-500/30' :
                      'bg-blue-500/10 text-blue-400 border-blue-500/30'
                    }`}>
                      {task.status === 'COMPLETED' && <CheckCircle2 className="w-2.5 h-2.5" />}
                      {task.status === 'WAITING_APPROVAL' && <ShieldAlert className="w-2.5 h-2.5" />}
                      {task.status === 'FAILED' && <XCircle className="w-2.5 h-2.5" />}
                      <span>{task.status}</span>
                    </span>
                  </td>
                  <td className="py-3 px-4 font-mono text-[11px]">
                    <span className={`px-1.5 py-0.5 rounded border text-[10px] font-semibold ${
                      task.risk_level === 'READ_ONLY' ? 'bg-slate-800 text-slate-300 border-slate-700' :
                      task.risk_level === 'LOW' ? 'bg-blue-500/10 text-blue-400 border-blue-500/20' :
                      task.risk_level === 'MEDIUM' ? 'bg-amber-500/10 text-amber-300 border-amber-500/30' :
                      'bg-red-500/10 text-red-400 border-red-500/30'
                    }`}>
                      {task.risk_level}
                    </span>
                  </td>
                  <td className="py-3 px-4 text-slate-500 font-mono text-[11px]">
                    {new Date(task.created_at).toLocaleTimeString()}
                  </td>
                  <td className="py-3 px-4 text-right">
                    <button className="p-1 rounded hover:bg-slate-800 text-slate-400 hover:text-white">
                      <ArrowUpRight className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};
