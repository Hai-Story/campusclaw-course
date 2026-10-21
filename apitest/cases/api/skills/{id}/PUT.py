from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    path=f'/api/skills/{c.skill["id"]}'
    c.expect('PUT',path,{'source':c.source},403,c.student)
    c.expect('PUT',path,{'source':'invalid'},400)
    assert c.scalar('SELECT source_text FROM tutor_skills WHERE id=%s',(c.skill['id'],))==c.source
    data=c.expect('PUT',path,{'source':c.source.replace('启用：是','启用：否')})
    assert not data['enabled'] and c.scalar('SELECT enabled FROM tutor_skills WHERE id=%s',(c.skill['id'],))==0
    c.expect('PUT',path,{'source':c.source})
    c.same404('PUT',f'/api/skills/{c.b_skill["id"]}','/api/skills/99999999',payload={'source':c.source})

if __name__ == "__main__": standalone(run)
