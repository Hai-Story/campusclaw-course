from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    data=c.expect('GET',f'/api/agents/{c.agent["id"]}',token=c.student)
    assert data['prompt']==c.scalar('SELECT prompt FROM tutor_agents WHERE id=%s',(data['id'],))
    assert data['skill_ids']==[c.skill['id']] and data['material_ids']==[c.material]
    c.same404('GET',f'/api/agents/{c.b_agent["id"]}','/api/agents/99999999',c.student)
    c.expect('PUT',f'/api/agents/{c.agent["id"]}',dict(c.agent_input,enabled=False))
    try:c.same404('GET',f'/api/agents/{c.agent["id"]}','/api/agents/99999999',c.student)
    finally:c.expect('PUT',f'/api/agents/{c.agent["id"]}',c.agent_input)

if __name__ == "__main__": standalone(run)
