from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    data=c.expect('GET',f'/api/skills/{c.skill["id"]}',token=c.student)
    assert data['source']==c.scalar('SELECT source_text FROM tutor_skills WHERE id=%s',(data['id'],))
    c.same404('GET',f'/api/skills/{c.b_skill["id"]}','/api/skills/99999999',c.student)

if __name__ == "__main__": standalone(run)
