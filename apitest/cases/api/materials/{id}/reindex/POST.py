from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    before=c.rows('SELECT content,citations_json,skill_name,status FROM tutor_messages WHERE conversation_id=%s ORDER BY id',(c.conversation['id'],))
    c.expect('POST',f'/api/materials/{c.material}/reindex',{'strategy':{'mode':'hierarchy'}},202)
    c.wait_ready()
    assert c.rows('SELECT content,citations_json,skill_name,status FROM tutor_messages WHERE conversation_id=%s ORDER BY id',(c.conversation['id'],))==before
    c.expect('POST',f'/api/materials/{c.material}/reindex',{'strategy':{'mode':'auto'}},403,c.student)
    c.same404('POST',f'/api/materials/{c.b_material}/reindex','/api/materials/99999999/reindex',payload={'strategy':{'mode':'auto'}})

if __name__ == "__main__": standalone(run)
