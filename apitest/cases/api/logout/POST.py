from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    token=c.login('teacher_a');c.tokens.append(token)
    before=c.scalar('SELECT COUNT(*) FROM sessions')
    c.expect('POST','/api/logout',status=204,token=token)
    assert c.scalar('SELECT COUNT(*) FROM sessions')==before-1
    assert c.call('GET','/api/me',token=token)[0]==401

if __name__ == "__main__": standalone(run)
