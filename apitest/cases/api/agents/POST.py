from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    before=c.scalar('SELECT COUNT(*) FROM tutor_agents')
    c.expect('POST','/api/agents',c.agent_input,403,c.student)
    assert c.scalar('SELECT COUNT(*) FROM tutor_agents')==before
    c.expect('POST','/api/agents',c.agent_input,400)
    for invalid in [dict(c.agent_input,name='字'*65),dict(c.agent_input,name=''),dict(c.agent_input,prompt='字'*2001),dict(c.agent_input,enabled='yes')]:c.expect('POST','/api/agents',invalid,400)
    cross=c.call('POST','/api/agents',dict(c.agent_input,name='cross'+c.suffix,material_ids=[c.b_material]),c.teacher)
    missing=c.call('POST','/api/agents',dict(c.agent_input,name='cross'+c.suffix,material_ids=[99999999]),c.teacher)
    assert cross[:2]==missing[:2] and cross[0]==404
    assert c.scalar('SELECT COUNT(*) FROM tutor_agents')==before
    data=c.expect('POST','/api/agents',dict(c.agent_input,name='空助手'+c.suffix,prompt='',skill_ids=[],material_ids=[]),201)
    row=c.rows('SELECT * FROM tutor_agents WHERE id=%s',(data['id'],))[0]
    assert row['prompt']=='' and row['class_id']==c.a['class_id'] and data['material_ids']==[]

if __name__ == "__main__": standalone(run)
