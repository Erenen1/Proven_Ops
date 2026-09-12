import React from 'react';
import { Agent } from '../types';
import { Server, Cpu, HardDrive, CheckCircle2, XCircle, Shield } from 'lucide-react';

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
            Install the OpsPilot Server Agent on your Ubuntu server using the bootstrap token.
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
                  <div className="pt-2 border-t border-slate-800/80 space-y-2 text-xs font-mono">
                    <div className="flex justify-between text-[11px]">
                      <span className="text-slate-400 flex items-center space-x-1">
                        <Cpu className="w-3 h-3 text-blue-400" />
                        <span>CPU: {agent.last_metrics.cpu_usage_percent.toFixed(1)}%</span>
                      </span>
                      <span className="text-slate-400 flex items-center space-x-1">
                        <HardDrive className="w-3 h-3 text-amber-400" />
                        <span>Disk: {agent.last_metrics.disk_usage_percent.toFixed(1)}%</span>
                      </span>
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
