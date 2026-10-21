from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == 'apitest')))
from support import standalone

def run(c):
    raw='---\nname: standard-preview\ndescription: 数学步骤提示\nallowed-tools: shell\n---\n先检查步骤。'
    before=c.scalar('SELECT COUNT(*) FROM tutor_skills')
    result=c.expect('POST','/api/skills/preview',{'source':raw})
    assert result['format']=='skill-md' and result['name']=='standard-preview'
    assert c.scalar('SELECT COUNT(*) FROM tutor_skills')==before
    c.expect('POST','/api/skills/preview',{'source':raw},403,c.student)
    c.expect('POST','/api/skills/preview',{'source':raw.replace('description:', 'name:')},400)
    course=c.expect('POST','/api/skills/preview',{'source':c.source})
    assert course['format']=='course'

if __name__=='__main__':standalone(run)
