from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    data=c.expect('GET','/api/me',token=c.student)['user']
    row=c.rows('SELECT role,class_id FROM users WHERE id=%s',(data['id'],))[0]
    assert data['role']==row['role'] and data['class_id']==row['class_id']
    assert c.call('GET','/api/me',headers={'Cookie':'campus_session=old'})[0]==401
    assert c.call('GET','/api/me',token=c.student[:-1]+'x')[0]==401

if __name__ == "__main__": standalone(run)
