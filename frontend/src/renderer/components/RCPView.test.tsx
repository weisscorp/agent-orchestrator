import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, expect, it, vi } from 'vitest';
import type { TaskComposerProps } from './TaskComposer';

const h=vi.hoisted(()=>({get:vi.fn(),composer:vi.fn(),projects:[] as unknown[]}));
vi.mock('../lib/api-client',()=>({apiClient:{GET:h.get},apiErrorMessage:()=>''}));
vi.mock('../hooks/useWorkspaceQuery',()=>({useWorkspaceQuery:()=>({data:h.projects})}));
vi.mock('@tanstack/react-router',()=>({useNavigate:()=>vi.fn()}));
vi.mock('./TaskComposer',()=>({TaskComposer:(props:TaskComposerProps)=>{h.composer(props);return <div data-testid="composer"/>;}}));
import { RCPView } from './RCPView';

const repository={id:'api',name:'example-org/api',url:'https://github.com/example-org/api'};
const project={id:'workspace',name:'github',kind:'workspace',path:'/workspace',repo:'',folderMissing:false,workspaceRepos:[{name:'api',relativePath:'api',repo:'git@github.com:example-org/api.git'}]};
const feature={id:'feature',product_id:'product',title:'Plan routes',status:'planned',repositories:['api']};
const context={feature,integrations:[{id:'integration',feature_id:'feature',title:'Inspect',status:'planned',repositories:['api']}],repositories:[repository],memories:[]};
beforeEach(()=>{
 h.composer.mockClear();h.get.mockReset();h.projects=[project,{id:'standalone',kind:'standalone'},{...project,id:'remote',hostId:'host'}];
 h.get.mockImplementation(async (path:string)=>({data:path==='/api/v1/rcp/features'?{features:[feature],repositories:[repository]}:path==='/api/v1/rcp/features/{featureId}'?context:{status:'ok',project}}));
});
async function selectRepository(){
 render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><RCPView/></QueryClientProvider>);
 fireEvent.click(await screen.findByRole('button',{name:/Plan routes/}));
 fireEvent.change(await screen.findByRole('combobox',{name:'Этап'}),{target:{value:'integration'}});
 fireEvent.change(screen.getByRole('combobox',{name:'Репозиторий'}),{target:{value:'api'}});
}
it('prepares a registered workspace child with exactly that child selected',async()=>{
 await selectRepository();
 const button=screen.getByRole('button',{name:'Подготовить задачу'});
 await waitFor(()=>expect(button).toBeEnabled());fireEvent.click(button);
 expect(await screen.findByTestId('composer')).toBeInTheDocument();
 const props=h.composer.mock.lastCall![0] as TaskComposerProps;
 expect(props.projectId).toBe('workspace');expect(props.initialWorkspaceRepos).toEqual(['api']);
 await expect(props.prepareBrief!('Inspect',['api'])).resolves.toContain("git -C 'api' remote get-url origin");
 h.get.mockResolvedValueOnce({data:context}).mockResolvedValueOnce({data:{status:'ok',project:{...project,workspaceRepos:[{...project.workspaceRepos[0],repo:'https://github.com/other/api'}]}}});
 await expect(props.prepareBrief!('Inspect',['api'])).rejects.toThrow();
});
it('fails closed for ambiguous matching workspace projects',async()=>{
 h.projects=[project,{...project,id:'duplicate'}];
 await selectRepository();
 await waitFor(()=>expect(h.get.mock.calls.filter(([path])=>path==='/api/v1/projects/{id}')).toHaveLength(2));
 expect(screen.getByRole('button',{name:'Подготовить задачу'})).toBeDisabled();
 expect(h.composer).not.toHaveBeenCalled();
});
