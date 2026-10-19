import { describe, expect, it, vi } from 'vitest';
vi.mock('./api-client',()=>({apiClient:{GET:vi.fn()},apiErrorMessage:()=>''}));
import { apiClient } from './api-client';
import type { components } from '../../api/schema';
import { canonicalRCPRepository, prepareRCPPlanningBrief, rcpPlanningTargets, rcpPlanningBrief, type RCPContext } from './rcp';

const context: RCPContext={
 feature:{id:'feature',product_id:'product',title:'Feature',status:'planned',goal:'Plan',problem:'',requirements:[],constraints:[],repositories:['back','front']},
 integrations:[{id:'integration',feature_id:'feature',title:'Read-only plan',objective:'Plan',status:'planned',owner:'',repositories:['back'],acceptance_criteria:[],completed:[],remaining:[]}],
 repositories:[],memories:[],history_total:0,history_truncated:false,
};
describe('RCP task context',()=>{
 it('preserves feature/integration/repository and requires fresh resume plus durable handoff',()=>{
  const text=rcpPlanningBrief(context,'integration','back','Inspect routes');
  for(const value of ['Product: product','Feature: feature','Integration: integration','Repository: back','read-only planning','resume','record_progress','handoff','AO_SESSION_ID','git remote get-url origin','При несовпадении','Inspect routes'])expect(text).toContain(value);
  expect(rcpPlanningBrief(context,'integration','back','Inspect routes')).toBe(text);
 });
 it('rejects wrong feature, repository and already claimed integration',()=>{
  expect(()=>rcpPlanningBrief(context,'unknown','back','')).toThrow();
  expect(()=>rcpPlanningBrief(context,'integration','unrelated','')).toThrow();
  expect(()=>rcpPlanningBrief(context,'integration','front','')).toThrow();
  expect(()=>rcpPlanningBrief({...context,integrations:[{...context.integrations[0],feature_id:'other'}]},'integration','back','')).toThrow();
  expect(()=>rcpPlanningBrief({...context,integrations:[{...context.integrations[0],status:'working'}]},'integration','back','')).toThrow();
 });
 it('normalizes a real origin and rejects unrelated or local paths',()=>{
  expect(canonicalRCPRepository('git@github.com:example-org/backend.git')).toBe(canonicalRCPRepository('https://github.com/example-org/backend'));
  expect(canonicalRCPRepository('https://github.com/other/backend')).not.toBe(canonicalRCPRepository('https://github.com/example-org/backend'));
  expect(canonicalRCPRepository('/tmp/backend')).toBeUndefined();
 });
});

const repository: components['schemas']['RCPRepository']={id:'back',name:'example-org/api',url:'https://github.com/example-org/api.git'};
const workspace: components['schemas']['Project']={id:'workspace',name:'github',kind:'workspace',repo:'',path:'/workspace',defaultBranch:'main',folderMissing:false,workspaceRepos:[{name:'api',relativePath:'services/api',repo:'git@github.com:example-org/api.git'},{name:'web',relativePath:'web',repo:'https://github.com/example-org/web'}]};
describe('workspace RCP binding',()=>{
 it('matches a child by name and origin, preserves standalone origins, and exposes ambiguity',()=>{
  const target=rcpPlanningTargets([workspace],repository);
  expect(target).toEqual([{projectId:'workspace',origin:'github.com/example-org/api',workspaceRepo:{name:'api',relativePath:'services/api'}}]);
  expect(rcpPlanningTargets([{...workspace,workspaceRepos:[{name:'api',relativePath:'api',repo:'https://github.com/other/api'}]}],repository)).toEqual([]);
  expect(rcpPlanningTargets([{...workspace,workspaceRepos:[{name:'other',relativePath:'api',repo:repository.url}]}],repository)).toEqual([]);
  expect(rcpPlanningTargets([{...workspace,kind:'single_repo',repo:repository.url}],repository)).toEqual([{projectId:'workspace',origin:'github.com/example-org/api'}]);
  expect(rcpPlanningTargets([workspace,{...workspace,id:'duplicate'}],repository)).toHaveLength(2);
 });
 it('revalidates fresh origin, path, selection and integration before preparing',async()=>{
  const get=vi.mocked(apiClient.GET);
  const fresh={...context,repositories:[repository]};
  const target=rcpPlanningTargets([workspace],repository)[0];
  const respond=(project=workspace,ctx=fresh)=>get.mockImplementation(async (path)=>path==='/api/v1/rcp/features/{featureId}'?{data:ctx,error:undefined,response:new Response()}:{data:{status:'ok',project},error:undefined,response:new Response()} as never);
  respond();
  const brief=await prepareRCPPlanningBrief('feature','integration','back','workspace','Inspect',target,['api']);
  expect(brief).toContain("git -C 'services/api' remote get-url origin");
  expect(brief).toContain('services/api');
  for(const selection of [undefined,[],['web'],['api','web']])await expect(prepareRCPPlanningBrief('feature','integration','back','workspace','',target,selection)).rejects.toThrow();
  respond({...workspace,workspaceRepos:[{...workspace.workspaceRepos![0],relativePath:'replacement'}]});
  await expect(prepareRCPPlanningBrief('feature','integration','back','workspace','',target,['api'])).rejects.toThrow();
  respond({...workspace,workspaceRepos:[{...workspace.workspaceRepos![0],repo:'https://github.com/other/api'}]});
  await expect(prepareRCPPlanningBrief('feature','integration','back','workspace','',target,['api'])).rejects.toThrow();
  respond(workspace,{...fresh,integrations:[{...fresh.integrations[0],status:'working'}]});
  await expect(prepareRCPPlanningBrief('feature','integration','back','workspace','',target,['api'])).rejects.toThrow();
 });
});
