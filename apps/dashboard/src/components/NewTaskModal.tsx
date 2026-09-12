import React, { useState } from 'react';
import { Agent } from '../types';
import { createTask } from '../api';
import { X, Sparkles, Server } from 'lucide-react';

interface NewTaskModalProps {
  agents: Agent[];
  onClose: () => void;
  onTaskCreated: (taskID: string) => void;
}

export const NewTaskModal: React.FC<NewTaskModalProps> = ({ agents, onClose, onTaskCreated }) => {
  const [prompt, setPrompt] = useState('');
  const [title, setTitle] = useState('');
  const [selectedAgent, setSelectedAgent] = useState<string>(
    agents.length > 0 ? agents[0].id : ''
  );
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!prompt.trim()) return;

    setIsSubmitting(true);
    try {
      const task = await createTask({
        title: title.trim() || undefined,
        prompt: prompt.trim(),
        target_agent_ids: selectedAgent ? [selectedAgent] : [],
      });
      onTaskCreated(task.id);
    } catch (err: any) {
      alert(err.message || 'Failed to submit task');
    } finally {
      setIsSubmitting(false);
    }
  };

  const templates = [
    "Install nginx on server X and expose it on port 8080",
    "Install Docker and verify daemon status",
    "payment-service neden başlamıyor, problemi bul ve çözüm öner",
    "Nginx kur, 8080 portunda çalıştır ve gerçekten erişilebilir olduğunu doğrula",
  ];

  return (
    <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="bg-slate-900 border border-slate-800 rounded-xl w-full max-w-lg shadow-2xl overflow-hidden">
        <div className="px-6 py-4 border-b border-slate-800 flex items-center justify-between bg-slate-950/40">
          <div className="flex items-center space-x-2">
            <Sparkles className="w-5 h-5 text-blue-400" />
            <h3 className="text-sm font-bold text-white">Create Infrastructure Task</h3>
          </div>
          <button onClick={onClose} className="text-slate-400 hover:text-white">
            <X className="w-5 h-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-4">
          {/* Target Host Selection */}
          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5 flex items-center space-x-1.5">
              <Server className="w-3.5 h-3.5 text-blue-400" />
              <span>Target Linux Host</span>
            </label>
            <select
              value={selectedAgent}
              onChange={(e) => setSelectedAgent(e.target.value)}
              className="w-full px-3 py-2 text-xs bg-slate-950 border border-slate-700 rounded-md text-slate-200 focus:outline-none focus:border-blue-500 font-mono"
            >
              {agents.length === 0 ? (
                <option value="">No agents online (will fail if none enrolled)</option>
              ) : (
                agents.map((agent) => (
                  <option key={agent.id} value={agent.id}>
                    {agent.hostname} ({agent.ip_address}) - {agent.distribution} {agent.version} [{agent.status}]
                  </option>
                ))
              )}
            </select>
          </div>

          {/* Optional Title */}
          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5">
              Task Title <span className="text-slate-500 font-normal">(optional)</span>
            </label>
            <input
              type="text"
              placeholder="e.g. Bootstrap Nginx on Port 8080"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              className="w-full px-3 py-2 text-xs bg-slate-950 border border-slate-700 rounded-md text-slate-200 focus:outline-none focus:border-blue-500"
            />
          </div>

          {/* Prompt / Intent */}
          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5">
              Natural Language Intent
            </label>
            <textarea
              rows={4}
              required
              placeholder="Describe your infrastructure task in plain language (e.g. Install nginx on server X and expose it on port 8080)..."
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              className="w-full px-3 py-2 text-xs bg-slate-950 border border-slate-700 rounded-md text-slate-200 focus:outline-none focus:border-blue-500 font-mono"
            />
          </div>

          {/* Templates */}
          <div>
            <span className="text-[10px] uppercase font-bold text-slate-500 tracking-wider">
              Example Scenarios:
            </span>
            <div className="mt-1.5 flex flex-wrap gap-1.5">
              {templates.map((tpl, i) => (
                <button
                  type="button"
                  key={i}
                  onClick={() => setPrompt(tpl)}
                  className="px-2.5 py-1 text-[11px] rounded bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 transition-colors text-left"
                >
                  {tpl}
                </button>
              ))}
            </div>
          </div>

          {/* Actions */}
          <div className="pt-2 flex justify-end space-x-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-xs text-slate-400 hover:text-white transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSubmitting || !prompt.trim()}
              className="px-4 py-2 text-xs font-bold bg-blue-600 hover:bg-blue-500 text-white rounded-md transition-colors disabled:opacity-50"
            >
              {isSubmitting ? 'Starting Plan...' : 'Plan & Execute Task'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
