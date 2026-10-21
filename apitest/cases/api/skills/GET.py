from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    items=c.expect('GET','/api/skills',token=c.student)['skills']
    assert any(s['id']==c.skill['id'] and s['source']==c.source for s in items)
    assert all(s['class_id']==c.a['class_id'] for s in items)
    assert c.call('GET','/api/skills')[0]==401

if __name__ == "__main__": standalone(run)
