import userEvent from "@testing-library/user-event";
import { render as renderView, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import type { ProjectOrchestratorAction } from "../hooks/useProjectOrchestratorAction";
import { ProjectBoardActions } from "./ProjectBoardActions";
import { TooltipProvider } from "./ui/tooltip";

const render = (view: ReactNode) => renderView(<QueryClientProvider client={new QueryClient()}>{view}</QueryClientProvider>);

const actions: ProjectOrchestratorAction = {
	projectId: undefined,
	orchestrator: undefined,
	isSpawning: false,
	isProjectRestarting: false,
	isProvisioning: false,
	spawnError: "",
	canCreateAsTui: false,
	openNewTask: vi.fn(),
	openOrchestrator: vi.fn(),
};

describe("ProjectBoardActions", () => {
	it("starts a workspace orchestrator with only the root after an explicit selection", async () => {
		const openOrchestrator = vi.fn();
		render(<TooltipProvider><ProjectBoardActions actions={{ ...actions, projectId: "workspace", openOrchestrator }} placement="empty"
			project={{ kind: "workspace", workspaceRepos: [{ name: "api" }] }} /></TooltipProvider>);
		await userEvent.click(screen.getByRole("button", { name: "Session repositories" }));
		await userEvent.click(screen.getByRole("button", { name: "Workspace root only" }));
		await userEvent.keyboard("{Escape}");
		await userEvent.click(screen.getByRole("button", { name: /spawn orchestrator/i }));
		expect(openOrchestrator).toHaveBeenCalledWith(undefined, []);
	});

	it("keeps the cloud Task plain without changing the Orchestrator button", () => {
		render(
			<TooltipProvider>
				<ProjectBoardActions actions={actions} cloud placement="header" />
			</TooltipProvider>,
		);
		expect(screen.getByRole("button", { name: "New task" })).toHaveClass("topbar-control--secondary");
		expect(screen.getByRole("button", { name: /orchestrator/i })).toHaveClass("topbar-control--primary", "bg-accent-strong");
	});

	it("preserves the existing empty local board controls", () => {
		render(
			<TooltipProvider>
				<ProjectBoardActions actions={actions} placement="header" quiet />
			</TooltipProvider>,
		);
		expect(screen.getByRole("button", { name: "New task" })).toHaveClass("topbar-control--secondary");
		expect(screen.getByRole("button", { name: /orchestrator/i })).toHaveClass("topbar-control--secondary");
	});
});
