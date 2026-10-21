from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if p.name=='apitest')))
from support import standalone

def run(c):
    items=c.expect('GET','/api/prompt-templates')['templates']
    assert len([x for x in items if x['builtin']])==3
    assert all(x['class_id']==c.a['class_id'] for x in items)
    c.expect('GET','/api/prompt-templates',status=403,token=c.student)
    assert all(x['class_id']==c.b['class_id'] for x in c.expect('GET','/api/prompt-templates',token=c.teacher_b)['templates'])
if __name__=='__main__':standalone(run)
