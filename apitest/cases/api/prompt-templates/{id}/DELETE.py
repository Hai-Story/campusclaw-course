from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if p.name=='apitest')))
from support import standalone

def run(c):
    builtin=next(x for x in c.expect('GET','/api/prompt-templates')['templates'] if x['builtin'])
    c.expect('DELETE',f"/api/prompt-templates/{builtin['id']}",status=403)
    item=c.expect('POST','/api/prompt-templates',dict(name='模板删除'+c.suffix,prompt='模板正文'),201)
    path=f"/api/prompt-templates/{item['id']}"
    c.expect('DELETE',path,status=403,token=c.student)
    c.same404('DELETE',path,'/api/prompt-templates/999999999',c.teacher_b)
    c.expect('DELETE',path,status=204)
    assert c.scalar('SELECT COUNT(*) FROM tutor_prompt_templates WHERE id=%s',(item['id'],))==0
    c.expect('DELETE',path,status=404)
if __name__=='__main__':standalone(run)
