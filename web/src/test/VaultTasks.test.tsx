import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import { ConfigProvider, App } from "antd";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import VaultTasks from "@/pages/vault/VaultTasks";
import { api } from "@/api";
import type { VaultTask } from "@/api";

vi.mock("react-virtuoso", () => ({
  Virtuoso: ({
    data,
    itemContent,
  }: {
    data: VaultTask[];
    itemContent: (index: number, task: VaultTask) => ReactNode;
  }) => (
    <div>{data.map((task, index) => <div key={task.id}>{itemContent(index, task)}</div>)}</div>
  ),
}));

vi.mock("@/pages/vault/TaskEditModal", () => ({
  default: () => null,
}));

vi.mock("@/api", () => ({
  api: {
    vaultTasks: {
      tasksList: vi.fn(),
      tasksPositionPartialUpdate: vi.fn(),
    },
    personalize: {
      personalizeDetail: vi.fn(),
    },
  },
}));

const tasks: VaultTask[] = [
  { id: 1, label: "Prepare launch", completed: false, status: "todo" },
  { id: 2, label: "Check rollout logs", completed: false, status: "todo", parent_task_id: 1 },
];

function renderVaultTasks() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });

  return render(
    <ConfigProvider>
      <App>
        <QueryClientProvider client={queryClient}>
          <MemoryRouter initialEntries={["/vaults/vault-1/tasks"]}>
            <Routes>
              <Route path="/vaults/:id/tasks" element={<VaultTasks />} />
            </Routes>
          </MemoryRouter>
        </QueryClientProvider>
      </App>
    </ConfigProvider>
  );
}

describe("VaultTasks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    vi.mocked(api.vaultTasks.tasksList).mockResolvedValue({
      data: tasks,
      success: true,
    });
    vi.mocked(api.personalize.personalizeDetail).mockResolvedValue({
      data: [
        {
          id: 1,
          slug: "todo",
          label: "Todo",
          position: 0,
          is_default: true,
          can_be_deleted: false,
        },
      ],
      success: true,
    });
  });

  it("marks sub-tasks differently from root tasks in the list view", async () => {
    renderVaultTasks();

    const rootTask = await screen.findByText("Prepare launch");
    const subTask = await screen.findByText("Check rollout logs");

    expect(rootTask.closest("[data-task-kind]")).toHaveAttribute("data-task-kind", "task");
    expect(subTask.closest("[data-task-kind]")).toHaveAttribute("data-task-kind", "sub-task");
  });

  it("marks sub-tasks differently from root tasks in the kanban view", async () => {
    renderVaultTasks();

    await userEvent.click(await screen.findByText("Kanban"));

    await waitFor(() => {
      expect(screen.getByText("Todo")).toBeInTheDocument();
    });

    const rootTask = screen.getByText("Prepare launch");
    const subTask = screen.getByText("Check rollout logs");

    expect(rootTask.closest("[data-task-kind]")).toHaveAttribute("data-task-kind", "task");
    expect(subTask.closest("[data-task-kind]")).toHaveAttribute("data-task-kind", "sub-task");
  });
});
