from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    items=c.expect('GET','/api/conversations',token=c.student)['conversations']
    assert any(x['id']==c.conversation['id'] for x in items)
    uid=c.expect('GET','/api/me',token=c.student)['user']['id']
    assert all(c.scalar('SELECT user_id FROM tutor_conversations WHERE id=%s',(x['id'],))==uid for x in items)
    assert all(x['id']!=c.conversation['id'] for x in c.expect('GET','/api/conversations')['conversations'])

if __name__ == "__main__": standalone(run)
