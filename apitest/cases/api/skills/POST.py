from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    before=c.scalar('SELECT COUNT(*) FROM tutor_skills')
    c.expect('POST','/api/skills',{'source':c.source},403,c.student)
    c.expect('POST','/api/skills',{'source':c.source},400)
    for text in [c.source.replace('启用：是','启用：maybe'),c.source.replace('knowledge_search','shell'),'名称：重复\n'+c.source,c.source.replace('名称：','未知：'),c.source.split('\n\n')[0]]:c.expect('POST','/api/skills',{'source':text},400)
    assert c.scalar('SELECT COUNT(*) FROM tutor_skills')==before
    row=c.rows('SELECT * FROM tutor_skills WHERE id=%s',(c.skill['id'],))[0]
    assert row['source_text']==c.source and row['enabled']==1


    raw='---\nname: '+('a'*54+c.suffix)+'\ndescription: 标准技能\nallowed-tools: shell\n---\n'+('先检查步骤。'*400)
    routing=dict(trigger='方程',subject='',tools=[],enabled=True)
    c.expect('POST','/api/skills',{'source':raw},400)
    item=c.expect('POST','/api/skills',{'source':raw,'routing':routing},201)
    assert len(item['name'])==64 and item['tools']==[] and item['source']==raw
    saved=c.expect('GET',f"/api/skills/{item['id']}")
    assert saved['source']==raw and saved['trigger']=='方程'
    disabled=c.expect('PUT',f"/api/skills/{item['id']}",{'source':raw,'routing':dict(routing,enabled=False)})
    assert disabled['enabled'] is False
    c.expect('POST','/api/skills',{'source':raw,'routing':dict(routing,tools=['shell'])},400)
    # A 64-character skill name must also persist in tutor_messages.skill_name.
    c.expect('PUT',f"/api/skills/{item['id']}",{'source':raw,'routing':routing})
    agent=c.expect('POST','/api/agents',dict(name='长名称验收'+c.suffix,prompt='',enabled=True,skill_ids=[item['id']],material_ids=[]),201)
    conv=c.expect('POST','/api/conversations',{'agent_id':agent['id']},201,c.student)
    c.assert_saved(c.stream('方程怎么移项',conversation=conv['id']))
    assert c.scalar('SELECT skill_name FROM tutor_messages WHERE conversation_id=%s AND role=\'assistant\'',(conv['id'],))==item['name']

if __name__ == "__main__": standalone(run)
