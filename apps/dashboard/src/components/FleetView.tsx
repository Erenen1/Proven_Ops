import React from 'react';
import { Agent } from '../types';
import { Server, Cpu, HardDrive, CheckCircle2, XCircle, Shield, Activity } from 'lucide-react';

interface FleetViewProps {
  agents: Agent[];
  onSelectAgent?: (agent: Agent) => void;
}

export const FleetView: React.FC<FleetViewProps> = ({ agents }) => {
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-white tracking-tight">Managed Fleet</h2>
          <p className="text-xs text-slate-400">
            Registered Linux hosts connected via outbound mTLS gRPC tunnel
          </p>
        </div>
        <span className="text-xs font-mono text-slate-400">
          Total Nodes: <strong className="text-white">{agents.length}</strong>
        </span>
      </div>

      {agents.length === 0 ? (
        <div className="p-12 text-center rounded-lg bg-slate-900 border border-slate-800">
          <Server className="w-12 h-12 text-slate-600 mx-auto mb-3" />
          <h3 className="text-sm font-semibold text-slate-300">No Agents Enrolled</h3>
          <p className="text-xs text-slate-500 mt-1 max-w-md mx-auto">
            Install the ProvenOps Server Agent on your Ubuntu server using the bootstrap token.
          </p>
          <code className="mt-4 inline-block px-3 py-1.5 rounded bg-slate-950 border border-slate-800 text-[11px] font-mono text-blue-400">
            curl -sSL http://&lt;control-plane&gt;:8080/scripts/install-agent.sh | sudo bash
          </code>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {agents.map((agent) => {
            const isOnline = agent.status === 'online';
            return (
              <div
                key={agent.id}
                className="p-5 rounded-lg bg-slate-900 border border-slate-800 hover:border-slate-700 transition-colors space-y-4"
              >
                {/* Host Header */}
                <div className="flex items-start justify-between">
                  <div className="flex items-center space-x-3">
                    <div className={`p-2 rounded-lg border ${
                      isOnline ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400' : 'bg-slate-800 border-slate-700 text-slate-500'
                    }`}>
                      <Server className="w-5 h-5" />
                    </div>
                    <div>
                      <h4 className="text-sm font-bold text-white font-mono">{agent.hostname}</h4>
                      <p className="text-xs text-slate-400">{agent.ip_address} &bull; {agent.environment}</p>
                    </div>
                  </div>

                  <span className={`flex items-center space-x-1.5 text-[11px] font-semibold px-2 py-0.5 rounded-full border ${
                    isOnline
                      ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
                      : 'bg-red-500/10 text-red-400 border-red-500/30'
                  }`}>
                    {isOnline ? <CheckCircle2 className="w-3 h-3" /> : <XCircle className="w-3 h-3" />}
                    <span className="capitalize">{agent.status}</span>
                  </span>
                </div>

                {/* System Specs */}
                <div className="grid grid-cols-2 gap-2 text-xs py-2 border-y border-slate-800/80 font-mono">
                  <div>
                    <span className="text-slate-500 text-[10px] uppercase">OS / Distro:</span>
                    <p className="text-slate-300 capitalize">{agent.distribution} {agent.version}</p>
                  </div>
                  <div>
                    <span className="text-slate-500 text-[10px] uppercase">Architecture:</span>
                    <p className="text-slate-300">{agent.architecture}</p>
                  </div>
                </div>

                {/* Capabilities Badges */}
                <div>
                  <span className="text-[10px] uppercase font-bold text-slate-500 tracking-wider">
                    Reported Capabilities:
                  </span>
                  <div className="mt-1.5 flex flex-wrap gap-1.5">
                    {agent.capabilities && agent.capabilities.length > 0 ? (
                      agent.capabilities.map((cap) => (
                        <span
                          key={cap}
                          className="px-2 py-0.5 rounded text-[10px] font-mono bg-slate-800 text-slate-300 border border-slate-700 flex items-center space-x-1"
                        >
                          <Shield className="w-2.5 h-2.5 text-blue-400" />
                          <span>{cap}</span>
                        </span>
                      ))
                    ) : (
                      <span className="text-[11px] text-slate-500">None reported</span>
                    )}
                  </div>
                </div>

                {/* Live Resource Gauges (if available) */}
                {agent.last_metrics && (
                  <div className="pt-3 border-t border-slate-800/80 space-y-2.5 text-xs font-mono">
                    {/* CPU Usage */}
                    <div className="space-y-1">
                      <div className="flex justify-between text-[11px]">
                        <span className="text-slate-400 flex items-center space-x-1.5">
                          <Cpu className="w-3 h-3 text-blue-400" />
                          <span>CPU Usage</span>
                        </span>
                        <span className={`font-semibold ${
                          agent.last_metrics.cpu_usage_percent > 85 ? 'text-red-400' :
                          agent.last_metrics.cpu_usage_percent > 65 ? 'text-amber-400' :
                          'text-emerald-400'
                        }`}>
                          {agent.last_metrics.cpu_usage_percent.toFixed(1)}%
                        </span>
                      </div>
                      <div className="w-full bg-slate-950 rounded-full h-1.5 overflow-hidden border border-slate-800">
                        <div
                          className={`h-full transition-all duration-500 rounded-full ${
                            agent.last_metrics.cpu_usage_percent > 85 ? 'bg-red-500' :
                            agent.last_metrics.cpu_usage_percent > 65 ? 'bg-amber-500' :
                            'bg-blue-500'
                          }`}
                          style={{ width: `${Math.min(100, Math.max(0, agent.last_metrics.cpu_usage_percent))}%` }}
                        />
                      </div>
                    </div>

                    {/* Memory Usage */}
                    {agent.last_metrics.memory_total_bytes > 0 && (
                      <div className="space-y-1">
                        <div className="flex justify-between text-[11px]">
                          <span className="text-slate-400 flex items-center space-x-1.5">
                            <Activity className="w-3 h-3 text-purple-400" />
                            <span>Memory</span>
                          </span>
                          <span className="text-slate-300">
                            {(agent.last_metrics.memory_usage_bytes / (1024 * 1024 * 1024)).toFixed(1)}GB / {(agent.last_metrics.memory_total_bytes / (1024 * 1024 * 1024)).toFixed(1)}GB ({((agent.last_metrics.memory_usage_bytes / agent.last_metrics.memory_total_bytes) * 100).toFixed(0)}%)
                          </span>
                        </div>
                        <div className="w-full bg-slate-950 rounded-full h-1.5 overflow-hidden border border-slate-800">
                          <div
                            className="h-full bg-purple-500 transition-all duration-500 rounded-full"
                            style={{ width: `${Math.min(100, Math.max(0, (agent.last_metrics.memory_usage_bytes / agent.last_metrics.memory_total_bytes) * 100))}%` }}
                          />
                        </div>
                      </div>
                    )}

                    {/* Disk & Load Average */}
                    <div className="grid grid-cols-2 gap-2 pt-1">
                      <div className="bg-slate-950/60 p-2 rounded border border-slate-800/80">
                        <div className="flex items-center justify-between text-[10px] text-slate-400">
                          <span className="flex items-center space-x-1">
                            <HardDrive className="w-2.5 h-2.5 text-amber-400" />
                            <span>Disk Root</span>
                          </span>
                          <span className="font-semibold text-slate-200">{agent.last_metrics.disk_usage_percent.toFixed(0)}%</span>
                        </div>
                        <div className="w-full bg-slate-900 rounded-full h-1 mt-1 overflow-hidden">
                          <div
                            className="h-full bg-amber-500 rounded-full"
                            style={{ width: `${Math.min(100, agent.last_metrics.disk_usage_percent)}%` }}
                          />
                        </div>
                      </div>

                      <div className="bg-slate-950/60 p-2 rounded border border-slate-800/80 flex flex-col justify-between text-[10px]">
                        <span className="text-slate-400">Load (1m):</span>
                        <span className="font-semibold text-slate-200 font-mono">
                          {agent.last_metrics.load_avg_1m ? agent.last_metrics.load_avg_1m.toFixed(2) : '0.00'}
                          {agent.last_metrics.active_tasks ? ` (${agent.last_metrics.active_tasks} active)` : ''}
                        </span>
                      </div>
                    </div>
                  </div>
                )}

                {/* Footer */}
                <div className="text-[10px] text-slate-500 flex items-center justify-between pt-1">
                  <span>ID: {agent.id}</span>
                  <span>Heartbeat: {agent.last_heartbeat ? new Date(agent.last_heartbeat).toLocaleTimeString() : 'N/A'}</span>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
