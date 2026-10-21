from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    path=f'/api/skills/{c.skill["id"]}'
    c.expect('DELETE',path,status=403,token=c.student)
    c.same404('DELETE',f'/api/skills/{c.b_skill["id"]}','/api/skills/99999999')
    temp=c.expect('POST','/api/skills',{'source':c.source.replace('名称：','名称：临时')},201)
    c.expect('PUT',f'/api/agents/{c.agent["id"]}',dict(c.agent_input,skill_ids=[c.skill['id'],temp['id']]))
    c.expect('DELETE',f'/api/skills/{temp["id"]}',status=204)
    assert c.scalar('SELECT COUNT(*) FROM tutor_skills WHERE id=%s',(temp['id'],))==0
    assert c.scalar('SELECT COUNT(*) FROM tutor_agent_skills WHERE skill_id=%s',(temp['id'],))==0
    assert c.scalar('SELECT skill_id FROM tutor_agent_skills WHERE agent_id=%s',(c.agent['id'],))==c.skill['id']

if __name__ == "__main__": standalone(run)
