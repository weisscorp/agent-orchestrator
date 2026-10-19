import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { RefreshCw, Workflow } from 'lucide-react';
import { apiClient, apiErrorMessage } from '../lib/api-client';
import { loadRCPContext, prepareRCPPlanningBrief } from '../lib/rcp';
import { useWorkspaceQuery } from '../hooks/useWorkspaceQuery';
import { TaskComposer } from './TaskComposer';
import { Button } from './ui/button';
import { Input } from './ui/input';

export function RCPView() {
 const navigate = useNavigate();
 const [search,setSearch] = useState('');
 const [selected,setSelected] = useState('');
 const [integrationId,setIntegrationId] = useState('');
 const [repositoryId,setRepositoryId] = useState('');
 const [compose,setCompose] = useState(false);
 const catalog = useQuery({ queryKey:['rcp-features'], staleTime:Infinity, retry:false, refetchOnWindowFocus:false, queryFn:async()=>{
  const {data,error}=await apiClient.GET('/api/v1/rcp/features');
  if(error||!data) throw new Error(apiErrorMessage(error,'RCP недоступен. Проверь подключение и повтори загрузку.'));
  return data;
 }});
 const context = useQuery({ queryKey:['rcp-feature',selected], enabled:!!selected, staleTime:0, retry:false, refetchOnWindowFocus:false, queryFn:()=>loadRCPContext(selected) });
 const workspaces = useWorkspaceQuery({includeCloud:false}).data??[];
 const feature=context.data?.feature;
 const integration=context.data?.integrations.find(item=>item.id===integrationId);
 const scope=new Set(integration?.repositories?.length?integration.repositories:[...(feature?.repositories??[]),...(context.data?.integrations.flatMap(item=>item.repositories??[])??[])]);
 const repositories=[...new Map([...(catalog.data?.repositories??[]),...(context.data?.repositories??[])].map(repo=>[repo.id,repo])).values()].filter(repo=>scope.has(repo.id));
 const repository=repositories.find(repo=>repo.id===repositoryId);
 const project=workspaces.find(item=>item.kind!=='cloud'&&(item.name===repository?.name||item.name===repository?.name.split('/').at(-1)));
 const visible=(catalog.data?.features??[]).filter(item=>`${item.title} ${item.product_id}`.toLocaleLowerCase().includes(search.toLocaleLowerCase()));
 const error=catalog.error??context.error;
 return <div className="flex min-h-0 flex-1 flex-col bg-background">
  <header className="workspace-topbar-container center-panel-titlebar flex h-toolbar shrink-0 items-center justify-between border-b border-border-strong px-4">
   <h1 className="flex items-center gap-2 font-medium"><Workflow aria-hidden="true" className="size-4"/>Фичи RCP</h1>
   <Button variant="ghost" disabled={catalog.isFetching||context.isFetching} onClick={()=>{void catalog.refetch();if(selected)void context.refetch();}}><RefreshCw aria-hidden="true" className="size-4"/>Обновить</Button>
  </header>
  {error?<div role="alert" className="m-4 text-destructive">{error.message}</div>:null}
  <div className="grid min-h-0 flex-1 grid-cols-1 overflow-auto md:grid-cols-[minmax(16rem,1fr)_minmax(0,2fr)]">
   <section aria-label="Список фич" className="min-w-0 overflow-auto border-r border-border p-4">
    <Input aria-label="Найти фичу" placeholder="Найти фичу…" value={search} onChange={event=>setSearch(event.target.value)}/>
    {catalog.isPending?<p role="status" className="py-4 text-muted-foreground">Загружаем фичи из RCP…</p>:null}
    {!catalog.isPending&&!error&&!visible.length?<p className="py-4 text-muted-foreground">Фич не найдено. Измени поиск или обнови список.</p>:null}
    <ul className="mt-3 divide-y divide-border">{visible.map(item=><li key={item.id}><button type="button" aria-pressed={selected===item.id} className="w-full rounded-md px-3 py-3 text-left hover:bg-accent focus-visible:outline-2 focus-visible:outline-ring aria-pressed:bg-accent" onClick={()=>{setSelected(item.id);setIntegrationId('');setRepositoryId('');setCompose(false);}}><span className="block text-sm font-medium">{item.title}</span><span className="mt-1 block text-xs text-muted-foreground">{item.product_id} · {item.status}</span></button></li>)}</ul>
    {catalog.data?.truncated?<p className="py-3 text-sm text-muted-foreground">Показаны первые 200 фич. Полный список доступен в RCP.</p>:null}
   </section>
   <section aria-label="Контекст фичи" className="min-w-0 overflow-auto p-6">
    {!selected?<p className="text-muted-foreground">Выбери фичу, чтобы увидеть её план, репозитории и историю.</p>:context.isFetching?<p role="status">Читаем актуальный контекст…</p>:feature&&context.data?<>
     <h2 className="text-xl font-semibold">{feature.title}</h2><p className="mt-2 max-w-prose text-sm text-muted-foreground">{feature.goal}</p>
     <h3 className="mb-2 mt-6 font-medium">План</h3>
     <ul className="divide-y divide-border">{context.data.integrations.map(item=><li key={item.id} className="py-3"><p className="text-sm font-medium">{item.title}</p><p className="mt-1 text-sm text-muted-foreground">{item.objective}</p><p className="mt-1 text-xs text-muted-foreground">{item.status} · {item.owner||'Без исполнителя'}</p></li>)}</ul>
     <h3 className="mb-2 mt-6 font-medium">Репозитории</h3><p className="text-sm">{repositories.map(item=>item.name).join(', ')||'В этой фиче репозитории пока не указаны'}</p>
     <h3 className="mb-2 mt-6 font-medium">История разработки</h3>
     {!context.data.memories.length?<p className="text-sm text-muted-foreground">Записей пока нет. Результаты анализа появятся здесь после сохранения в RCP.</p>:<ul className="divide-y divide-border">{context.data.memories.map(item=><li key={item.id} className="py-3 text-sm"><p className="text-xs text-muted-foreground">{item.kind} · {item.created_at}</p><p className="mt-1 whitespace-pre-wrap break-words">{[item.title,item.body,item.reason,item.current,...(item.completed??[]),...(item.remaining??[]),...(item.next??[]),...(item.warnings??[])].filter(Boolean).join('\n')}</p></li>)}</ul>}
     {context.data.history_truncated?<p className="mt-2 text-sm text-muted-foreground">Последние 20 из {context.data.history_total} записей. Полная история остаётся в RCP.</p>:null}
     <h3 className="mb-2 mt-6 font-medium">Подготовить анализ</h3><p className="mb-3 text-sm text-muted-foreground">Выбери этап и репозиторий. Загрузка фичи не запускает агента.</p>
     <label className="block text-sm">Этап<select className="my-2 block w-full rounded-md border border-input bg-background p-2" value={integrationId} onChange={e=>{setIntegrationId(e.target.value);setRepositoryId('');setCompose(false);}}><option value="">Выбрать этап</option>{context.data.integrations.map(item=><option key={item.id} value={item.id} disabled={item.status!=='planned'}>{item.title} · {item.status}</option>)}</select></label>
     <label className="block text-sm">Репозиторий<select className="my-2 block w-full rounded-md border border-input bg-background p-2" value={repositoryId} onChange={e=>{setRepositoryId(e.target.value);setCompose(false);}}><option value="">Выбрать репозиторий</option>{repositories.map(item=><option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
     {repository&&!project?<p role="status" className="mb-3 text-sm text-muted-foreground">Добавь существующую рабочую копию {repository.name} в проекты AO.</p>:null}
     <Button disabled={!integration||!project||!repositoryId||compose} onClick={()=>setCompose(true)}>Подготовить задачу</Button>
     {compose&&project?<div className="mt-4"><TaskComposer key={`${selected}:${integrationId}:${repositoryId}`} projectId={project.id} initialPrompt="Составь план первого безопасного этапа. Только чтение исходников." createLabel="Начать анализ" prepareBrief={brief=>prepareRCPPlanningBrief(selected,integrationId,repositoryId,project.id,brief)} onCreated={sessionId=>void navigate({to:'/projects/$projectId/sessions/$sessionId',params:{projectId:project.id,sessionId}})}/></div>:null}
    </>:null}
   </section>
  </div>
 </div>;
}
