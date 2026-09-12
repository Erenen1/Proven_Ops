import React from 'react';
import { Runbook } from '../types';
import { BookOpen, Play } from 'lucide-react';

interface RunbooksViewProps {
  runbooks: Runbook[];
  onExecuteRunbook?: (runbook: Runbook) => void;
}

export const RunbooksView: React.FC<RunbooksViewProps> = ({ runbooks }) => {
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-white tracking-tight">Reusable Runbooks</h2>
          <p className="text-xs text-slate-400">
            Deterministic operations saved from verified tasks. Executes without LLM replanning overhead.
          </p>
        </div>
        <span className="text-xs font-mono text-slate-400">
          Total Runbooks: <strong className="text-white">{runbooks.length}</strong>
        </span>
      </div>

      {runbooks.length === 0 ? (
        <div className="p-12 text-center rounded-lg bg-slate-900 border border-slate-800">
          <BookOpen className="w-10 h-10 text-slate-600 mx-auto mb-2" />
          <h3 className="text-sm font-semibold text-slate-300">No Runbooks Saved Yet</h3>
          <p className="text-xs text-slate-500 mt-1">
            Execute any task successfully and click "Save as Runbook" in the task detail screen.
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {runbooks.map((rb) => (
            <div
              key={rb.id}
              className="p-5 rounded-lg bg-slate-900 border border-slate-800 space-y-3"
            >
              <div className="flex items-start justify-between">
                <div>
                  <div className="flex items-center space-x-2">
                    <span className="font-mono text-xs text-blue-400 font-bold">{rb.slug}</span>
                    <span className="text-[10px] px-1.5 py-0.2 rounded bg-slate-800 text-slate-400 border border-slate-700">
                      v{rb.latest_version}
                    </span>
                  </div>
                  <h4 className="text-sm font-bold text-white mt-1">{rb.title}</h4>
                  <p className="text-xs text-slate-400 mt-0.5">{rb.description}</p>
                </div>

                <button
                  onClick={() => alert(`Runbook ${rb.slug} loaded. You can trigger execution across target fleet nodes.`)}
                  className="px-3 py-1.5 rounded text-xs font-semibold bg-blue-600 hover:bg-blue-500 text-white flex items-center space-x-1.5 transition-colors"
                >
                  <Play className="w-3 h-3" />
                  <span>Execute</span>
                </button>
              </div>

              {rb.steps && (
                <div className="pt-2 border-t border-slate-800 text-xs font-mono text-slate-400">
                  <span>Deterministic steps ({rb.steps.length}): </span>
                  <div className="mt-1 space-y-1">
                    {rb.steps.slice(0, 3).map((s, i) => (
                      <div key={i} className="text-[11px] text-slate-500 flex items-center space-x-1">
                        <span>&bull;</span>
                        <span className="text-slate-300">{s.action}</span>
                      </div>
                    ))}
                    {rb.steps.length > 3 && (
                      <span className="text-[10px] text-slate-500">+{rb.steps.length - 3} more verified steps</span>
                    )}
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
