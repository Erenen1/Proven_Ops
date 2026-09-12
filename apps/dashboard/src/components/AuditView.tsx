import React, { useEffect, useState } from 'react';
import { AuditEvent } from '../types';
import { fetchAudit } from '../api';
import { Search } from 'lucide-react';

export const AuditView: React.FC = () => {
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [searchTerm, setSearchTerm] = useState('');

  useEffect(() => {
    fetchAudit(100).then(setEvents).catch(console.error);
  }, []);

  const filtered = events.filter((e) => {
    if (!searchTerm) return true;
    const term = searchTerm.toLowerCase();
    return (
      e.event_type.toLowerCase().includes(term) ||
      (e.action && e.action.toLowerCase().includes(term)) ||
      (e.task_id && e.task_id.toLowerCase().includes(term))
    );
  });

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h2 className="text-lg font-bold text-white tracking-tight">Audit Trail</h2>
          <p className="text-xs text-slate-400">
            Append-only immutable operational log of all tasks, approvals, and executions
          </p>
        </div>

        <div className="relative">
          <Search className="w-4 h-4 text-slate-500 absolute left-3 top-1/2 -translate-y-1/2" />
          <input
            type="text"
            placeholder="Search audit records..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="pl-9 pr-3 py-1.5 text-xs bg-slate-900 border border-slate-800 rounded-md text-slate-200 focus:outline-none focus:border-blue-500 w-64 font-mono"
          />
        </div>
      </div>

      <div className="rounded-lg bg-slate-900 border border-slate-800 overflow-hidden">
        <table className="w-full text-left text-xs">
          <thead className="bg-slate-950/60 border-b border-slate-800 text-slate-400 font-mono uppercase text-[10px]">
            <tr>
              <th className="py-3 px-4">Event & Action</th>
              <th className="py-3 px-4">Task / Target</th>
              <th className="py-3 px-4">User</th>
              <th className="py-3 px-4">Details</th>
              <th className="py-3 px-4 text-right">Timestamp</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800 font-mono">
            {filtered.length === 0 ? (
              <tr>
                <td colSpan={5} className="py-8 text-center text-slate-500">
                  No audit events found.
                </td>
              </tr>
            ) : (
              filtered.map((e) => (
                <tr key={e.id} className="hover:bg-slate-800/30 transition-colors">
                  <td className="py-3 px-4">
                    <span className="font-semibold text-blue-400">{e.event_type}</span>
                    {e.action && <span className="text-slate-400 block text-[11px]">{e.action}</span>}
                  </td>
                  <td className="py-3 px-4 text-slate-300 text-[11px]">
                    {e.task_id ? `${e.task_id.slice(0, 8)}...` : e.agent_id || 'System'}
                  </td>
                  <td className="py-3 px-4 text-slate-400">
                    {e.username || 'admin'}
                  </td>
                  <td className="py-3 px-4 text-[11px] text-slate-400 max-w-xs truncate">
                    {JSON.stringify(e.details)}
                  </td>
                  <td className="py-3 px-4 text-right text-slate-500 text-[11px]">
                    {new Date(e.created_at).toLocaleString()}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
};
