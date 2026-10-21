import { useState, type ReactElement } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { apiClient, apiErrorMessage } from "../lib/api-client";
import { clientForHost } from "../lib/host-clients";
import { Button } from "./ui/button";
import { ChevronDown } from "lucide-react";
import { Checkbox } from "./ui/checkbox";
import { Popover, PopoverContent, PopoverTrigger } from "./ui/popover";
import { SettingsMenuTrigger } from "./settings/SettingsMenuTrigger";

/** Undefined preserves the daemon's all-repository default; [] selects only the root. */
export function WorkspaceRepositoryPicker({ repositories, value, onChange, disabled, trigger, onStart, projectId, hostId }: {
	repositories?: { name: string }[];
	projectId?: string;
	hostId?: string;
	value?: string[];
	onChange: (names: string[] | undefined) => void;
	disabled?: boolean;
	trigger?: ReactElement;
	onStart?: () => void;
}) {
	const { t } = useTranslation();
	const [open, setOpen] = useState(false);
	const details = useQuery({
		queryKey: hostId ? ["project", hostId, projectId] : ["project", projectId],
		enabled: open && Boolean(projectId) && repositories === undefined,
		queryFn: async () => {
			const { data, error } = await (hostId ? clientForHost(hostId) : apiClient).GET("/api/v1/projects/{id}", {
				params: { path: { id: projectId ?? "" } },
			});
			if (error) throw new Error(apiErrorMessage(error));
			if (!data?.project) throw new Error(t("newTask.configUnavailable"));
			if ("resolveError" in data.project) throw new Error(data.project.resolveError);
			return data.project;
		},
	});
	const available = repositories ?? details.data?.workspaceRepos ?? [];
	const allLabel = t("newTask.allRepositories", { defaultValue: "All repositories" });
	const rootLabel = t("newTask.workspaceRootOnly", { defaultValue: "Workspace root only" });
	const summary = value === undefined ? allLabel : value.length === 0 ? rootLabel : value.join(", ");
	return <Popover open={open} onOpenChange={setOpen}>
		<PopoverTrigger asChild>
			{trigger ?? <SettingsMenuTrigger aria-label={t("newTask.sessionRepositories", { defaultValue: "Session repositories" })} title={summary} disabled={disabled} className="max-w-64 min-w-0">
				<span className="truncate">{summary}</span><ChevronDown className="size-icon-sm shrink-0" aria-hidden="true" />
			</SettingsMenuTrigger>}
		</PopoverTrigger>
		<PopoverContent align="start" className="w-64 p-2">
			<div className="mb-2 text-settings-muted">{t("newTask.workspaceRootIncluded", { defaultValue: "Workspace root is always included." })}</div>
			<label className="flex min-h-8 cursor-pointer items-center gap-2 text-settings-label">
				<Checkbox disabled={disabled} checked={value === undefined} onCheckedChange={(all) => onChange(all ? undefined : [])} />
				{allLabel}
			</label>
			<button type="button" disabled={disabled} className="min-h-8 text-settings-label hover:underline" onClick={() => onChange([])}>{rootLabel}</button>
			<div className="max-h-56 overflow-y-auto">
				{repositories === undefined && details.isFetching ? <div role="status" className="text-settings-muted">{t("common.loading")}</div> : null}
				{details.isError ? <div role="alert" className="text-settings-muted">
					{t("newTask.configUnavailable")}
					<button type="button" className="ml-2 min-h-8 hover:underline" disabled={details.isFetching} onClick={() => void details.refetch()}>{t("createProject.retry")}</button>
				</div> : null}
				{available.map(({ name }) => <label key={name} className="flex min-h-8 cursor-pointer items-center gap-2 text-settings-label">
					<Checkbox disabled={disabled} checked={value === undefined || value.includes(name)} onCheckedChange={(checked) => {
						const selected = value ?? available.map((repo) => repo.name);
						onChange(checked ? [...selected, name] : selected.filter((item) => item !== name));
					}} />
					<span className="min-w-0 truncate" title={name}>{name}</span>
				</label>)}
			</div>
			{onStart ? <Button className="mt-2 w-full" disabled={disabled} onClick={onStart}>{t("newTask.startOrchestrator", { defaultValue: "Start orchestrator" })}</Button> : null}
		</PopoverContent>
	</Popover>;
}

/** The sidebar icon opens explicit launch options before starting a workspace. */
export function WorkspaceOrchestratorLauncher({ enabled, repositories, children, onStart, disabled, projectId, hostId }: {
	enabled: boolean;
	repositories?: { name: string }[];
	projectId?: string;
	hostId?: string;
	children: ReactElement;
	onStart: (names?: string[]) => void;
	disabled?: boolean;
}) {
	const [names, setNames] = useState<string[]>();
	return enabled ? <WorkspaceRepositoryPicker projectId={projectId} hostId={hostId} repositories={repositories} value={names} onChange={setNames}
		trigger={children} disabled={disabled} onStart={() => onStart(names)} /> : children;
}
