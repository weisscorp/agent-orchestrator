import { describe, expect, it, vi } from 'vitest';
vi.mock('./api-client',()=>({apiClient:{},apiErrorMessage:()=>''}));
import { canonicalRCPRepository, rcpPlanningBrief, type RCPContext } from './rcp';

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
