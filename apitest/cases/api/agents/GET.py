from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    items=c.expect('GET','/api/agents',token=c.student)['agents']
    assert any(a['id']==c.agent['id'] for a in items) and all(a['enabled'] and a['class_id']==c.a['class_id'] for a in items)
    c.expect('PUT',f'/api/agents/{c.agent["id"]}',dict(c.agent_input,enabled=False))
    try:
        assert all(a['id']!=c.agent['id'] for a in c.expect('GET','/api/agents',token=c.student)['agents'])
        assert any(a['id']==c.agent['id'] and not a['enabled'] for a in c.expect('GET','/api/agents')['agents'])
    finally:c.expect('PUT',f'/api/agents/{c.agent["id"]}',c.agent_input)

if __name__ == "__main__": standalone(run)
