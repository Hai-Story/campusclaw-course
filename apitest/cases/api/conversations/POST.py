from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    before=c.scalar('SELECT COUNT(*) FROM tutor_conversations')
    c.expect('POST','/api/conversations',{},400,c.student)
    one=c.call('POST','/api/conversations',{'agent_id':c.b_agent['id']},c.student)
    two=c.call('POST','/api/conversations',{'agent_id':99999999},c.student)
    assert one[:2]==two[:2] and one[0]==404
    c.expect('PUT',f'/api/agents/{c.agent["id"]}',dict(c.agent_input,enabled=False))
    try:
        three=c.call('POST','/api/conversations',{'agent_id':c.agent['id']},c.student)
        assert three[:2]==two[:2]
    finally:c.expect('PUT',f'/api/agents/{c.agent["id"]}',c.agent_input)
    assert c.scalar('SELECT COUNT(*) FROM tutor_conversations')==before
    assert c.scalar('SELECT agent_id FROM tutor_conversations WHERE id=%s',(c.conversation['id'],))==c.agent['id']

if __name__ == "__main__": standalone(run)
