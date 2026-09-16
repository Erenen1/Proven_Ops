import React, { useEffect, useState } from 'react';
import { Agent, Task, Runbook } from './types';
import { fetchAgents, fetchTasks, fetchRunbooks } from './api';
import { Navbar } from './components/Navbar';
import { OverviewView } from './components/OverviewView';
import { FleetView } from './components/FleetView';
import { TasksView } from './components/TasksView';
import { ApprovalsView } from './components/ApprovalsView';
import { RunbooksView } from './components/RunbooksView';
import { AuditView } from './components/AuditView';
import { PoliciesView } from './components/PoliciesView';
import { NewTaskModal } from './components/NewTaskModal';
import { TaskDetailModal } from './components/TaskDetailModal';

export const App: React.FC = () => {
  const [activeTab, setActiveTab] = useState<string>('overview');
  const [agents, setAgents] = useState<Agent[]>([]);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [runbooks, setRunbooks] = useState<Runbook[]>([]);
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);
  const [isNewTaskOpen, setIsNewTaskOpen] = useState<boolean>(false);

  const loadAll = async () => {
    try {
      const [agentsData, tasksData, runbooksData] = await Promise.all([
        fetchAgents().catch(() => []),
        fetchTasks().catch(() => []),
        fetchRunbooks().catch(() => []),
      ]);
      setAgents(agentsData);
      setTasks(tasksData);
      setRunbooks(runbooksData);
    } catch (err) {
      console.error('Failed to load dashboard data:', err);
    }
  };

  useEffect(() => {
    loadAll();
    const interval = setInterval(loadAll, 3000);

    // Global SSE for instant fleet & task events
    const eventSource = new EventSource('/api/v1/events');
    eventSource.onmessage = () => {
      loadAll();
    };

    return () => {
      clearInterval(interval);
      eventSource.close();
    };
  }, []);

  const onlineAgentCount = agents.filter((a) => a.status === 'online').length;
  const pendingApprovalCount = tasks.filter((t) => t.status === 'WAITING_APPROVAL').length;

  const handleTaskCreated = (taskID: string) => {
    setIsNewTaskOpen(false);
    loadAll().then(() => {
      const created = tasks.find((t) => t.id === taskID);
      if (created) {
        setSelectedTask(created);
      }
    });
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col">
      <Navbar
        activeTab={activeTab}
        setActiveTab={setActiveTab}
        onlineAgentCount={onlineAgentCount}
        pendingApprovalCount={pendingApprovalCount}
        onNewTaskClick={() => setIsNewTaskOpen(true)}
      />

      <main className="flex-1 max-w-7xl w-full mx-auto p-6">
        {activeTab === 'overview' && (
          <OverviewView
            agents={agents}
            tasks={tasks}
            onSelectTask={(task) => setSelectedTask(task)}
            onNavigateTab={(tab) => setActiveTab(tab)}
            onNewTaskClick={() => setIsNewTaskOpen(true)}
          />
        )}

        {activeTab === 'fleet' && (
          <FleetView agents={agents} />
        )}

        {activeTab === 'tasks' && (
          <TasksView
            tasks={tasks}
            onSelectTask={(task) => setSelectedTask(task)}
            onNewTaskClick={() => setIsNewTaskOpen(true)}
          />
        )}

        {activeTab === 'approvals' && (
          <ApprovalsView
            tasks={tasks}
            onSelectTask={(task) => setSelectedTask(task)}
          />
        )}

        {activeTab === 'runbooks' && (
          <RunbooksView
            runbooks={runbooks}
            agents={agents}
            onTaskCreated={(newTask) => {
              setTasks((prev) => [newTask, ...prev]);
              setSelectedTask(newTask);
            }}
          />
        )}


        {activeTab === 'audit' && (
          <AuditView />
        )}

        {activeTab === 'policies' && (
          <PoliciesView />
        )}
      </main>

      {/* New Task Modal */}
      {isNewTaskOpen && (
        <NewTaskModal
          agents={agents}
          onClose={() => setIsNewTaskOpen(false)}
          onTaskCreated={handleTaskCreated}
        />
      )}

      {/* Task Detail Modal */}
      {selectedTask && (
        <TaskDetailModal
          task={selectedTask}
          onClose={() => setSelectedTask(null)}
          onTaskUpdated={loadAll}
        />
      )}
    </div>
  );
};
