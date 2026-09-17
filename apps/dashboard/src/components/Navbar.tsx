import React from 'react';
import { 
  Server, 
  Terminal, 
  ShieldCheck, 
  BookOpen, 
  History, 
  FileText, 
  PlusCircle, 
  Activity,
  Cpu
} from 'lucide-react';

interface NavbarProps {
  activeTab: string;
  setActiveTab: (tab: string) => void;
  onlineAgentCount: number;
  pendingApprovalCount: number;
  onNewTaskClick: () => void;
}

export const Navbar: React.FC<NavbarProps> = ({
  activeTab,
  setActiveTab,
  onlineAgentCount,
  pendingApprovalCount,
  onNewTaskClick,
}) => {
  const navItems = [
    { id: 'overview', label: 'Overview', icon: Activity },
    { id: 'fleet', label: 'Fleet', icon: Server, badge: onlineAgentCount },
    { id: 'tasks', label: 'Tasks', icon: Terminal },
    { 
      id: 'approvals', 
      label: 'Approvals', 
      icon: ShieldCheck, 
      badge: pendingApprovalCount, 
      badgeAlert: pendingApprovalCount > 0 
    },
    { id: 'runbooks', label: 'Runbooks', icon: BookOpen },
    { id: 'audit', label: 'Audit Trail', icon: History },
    { id: 'policies', label: 'Policies', icon: FileText },
  ];

  return (
    <header className="border-b border-slate-800 bg-slate-900/90 backdrop-blur sticky top-0 z-30 px-6 py-3">
      <div className="max-w-7xl mx-auto flex items-center justify-between">
        {/* Brand */}
        <div className="flex items-center space-x-3">
          <div className="w-8 h-8 rounded-lg bg-blue-600/20 border border-blue-500/30 flex items-center justify-center text-blue-400">
            <Cpu className="w-5 h-5" />
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <span className="font-bold text-lg tracking-tight text-white font-mono">ProvenOps</span>
              <span className="text-[10px] uppercase font-semibold tracking-wider px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-400 border border-blue-500/20">
                v1.0 Core
              </span>
            </div>
            <p className="text-xs text-slate-400 hidden sm:block">Policy-Controlled AI Infrastructure</p>
          </div>
        </div>

        {/* Navigation Tabs */}
        <nav className="flex items-center space-x-1">
          {navItems.map((item) => {
            const Icon = item.icon;
            const isActive = activeTab === item.id;
            return (
              <button
                key={item.id}
                onClick={() => setActiveTab(item.id)}
                className={`flex items-center space-x-2 px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
                  isActive
                    ? 'bg-slate-800 text-blue-400 border border-slate-700'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'
                }`}
              >
                <Icon className="w-4 h-4" />
                <span>{item.label}</span>
                {item.badge !== undefined && item.badge > 0 && (
                  <span
                    className={`px-1.5 py-0.2 text-[10px] rounded-full font-bold ${
                      item.badgeAlert
                        ? 'bg-amber-500/20 text-amber-300 border border-amber-500/40 animate-pulse'
                        : 'bg-slate-800 text-slate-300 border border-slate-700'
                    }`}
                  >
                    {item.badge}
                  </span>
                )}
              </button>
            );
          })}
        </nav>

        {/* Action Button */}
        <div className="flex items-center space-x-3">
          <button
            onClick={onNewTaskClick}
            className="flex items-center space-x-2 px-3.5 py-1.5 rounded-md text-xs font-semibold bg-blue-600 hover:bg-blue-500 text-white shadow-sm transition-colors border border-blue-400/20"
          >
            <PlusCircle className="w-4 h-4" />
            <span>New Task</span>
          </button>
        </div>
      </div>
    </header>
  );
};
