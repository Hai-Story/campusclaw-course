from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    path=f'/api/conversations/{c.conversation["id"]}'
    data=c.expect('GET',path,token=c.student)
    assert data['conversation']['id']==c.conversation['id']
    assert data['messages']==[]
    c.same404('GET',path,'/api/conversations/99999999',c.teacher)
    c.same404('GET',path,'/api/conversations/99999999',c.other)

if __name__ == "__main__": standalone(run)
