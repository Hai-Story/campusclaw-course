from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if p.name=='apitest')))
from support import standalone

def run(c):
    builtin=next(x for x in c.expect('GET','/api/prompt-templates')['templates'] if x['builtin'])
    body=dict(name='模板修改'+c.suffix,prompt='修改后的提示词')
    c.expect('PUT',f"/api/prompt-templates/{builtin['id']}",body,403)
    item=c.expect('POST','/api/prompt-templates',body,201)
    # Applying a template is a snapshot stored in agent.prompt.
    agent=c.expect('POST','/api/agents',dict(c.agent_input,name='模板助手'+c.suffix,prompt=item['prompt']),201)
    body['prompt']='再次修改'
    c.expect('PUT',f"/api/prompt-templates/{item['id']}",body)
    assert c.scalar('SELECT prompt FROM tutor_prompt_templates WHERE id=%s',(item['id'],))==body['prompt']
    assert c.expect('GET',f"/api/agents/{agent['id']}")['prompt']==item['prompt']
    c.expect('PUT',f"/api/prompt-templates/{item['id']}",body,403,c.student)
    c.same404('PUT',f"/api/prompt-templates/{item['id']}",'/api/prompt-templates/999999999',c.teacher_b,body)
if __name__=='__main__':standalone(run)
