from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if p.name=='apitest')))
from support import standalone

def run(c):
    payload=dict(name='自定义模板'+c.suffix,prompt='请逐步提示学生。')
    c.expect('POST','/api/prompt-templates',payload,403,c.student)
    item=c.expect('POST','/api/prompt-templates',payload,201)
    assert item['builtin'] is False and c.scalar('SELECT prompt FROM tutor_prompt_templates WHERE id=%s',(item['id'],))==payload['prompt']
    c.expect('POST','/api/prompt-templates',payload,400)
    for body in [dict(payload,name=''),dict(payload,prompt=' '),dict(payload,prompt='字'*2001)]:c.expect('POST','/api/prompt-templates',body,400)
if __name__=='__main__':standalone(run)
