"""Knowledge integration for actual preset apps and their current skill bindings."""
import time
from support import Context


def run(c):
    apps={a['name']:a for a in c.expect('GET','/api/agents')['agents']}
    cases=[('数学解题助手','数学','方程去括号需要检查什么？'),('语文阅读助手','语文','阅读文章应该怎么分析段落？'),('英语学习助手','英语','英语语法和时态应该怎么检查？')]
    materials=[]
    for _,subject,_ in cases:
        material=c.upload(subject+'预置应用验收-'+c.suffix,subject+'方程、阅读、语法的学习步骤：先检查条件与上下文，再核对每一步依据。')
        materials.append(material)
    for _ in range(100):
        ready=c.scalar("SELECT COUNT(DISTINCT material_id) FROM knowledge_chunks WHERE material_id IN (%s,%s,%s) AND index_status='ready'",tuple(materials))
        if ready==3:break
        time.sleep(.2)
    else:raise AssertionError('preset app materials not indexed')
    for (name,subject,question),material in zip(cases,materials):
        original=apps[name]
        payload={k:original[k] for k in ('name','prompt','enabled','skill_ids','material_ids')}
        try:
            # Empty binding produces an answer, with no evidence even when class materials exist.
            c.expect('PUT',f"/api/agents/{original['id']}",dict(payload,material_ids=[]))
            conv=c.expect('POST','/api/conversations',{'agent_id':original['id']},201,c.student)
            empty=c.stream(question,conversation=conv['id']);c.assert_saved(empty)
            assert empty[0][1]['citations']==[]
            # Reconfiguration is read on the next turn of the same conversation.
            c.expect('PUT',f"/api/agents/{original['id']}",dict(payload,material_ids=[material]))
            events=c.stream(question,conversation=conv['id']);text=c.assert_saved(events)
            hits=events[0][1]['citations']
            assert hits and {h['material_id'] for h in hits}=={material},(name,hits)
            assert all(subject in h['title'] and h['excerpt'] for h in hits)
            assert '[1]' in text
            assert all(h['material_id'] not in [c.unbound,c.b_material] for h in hits)
            c.expect('PUT',f"/api/agents/{original['id']}",dict(payload,material_ids=[c.b_material]),404)
            c.same404('GET',f"/api/conversations/{conv['id']}",'/api/conversations/999999999',token=c.other)
        finally:
            c.expect('PUT',f"/api/agents/{original['id']}",payload)
    print('PASS preset app knowledge: three subjects, empty/bound evidence, live configuration, class isolation')

if __name__=='__main__':
    c=Context()
    try:run(c)
    finally:c.close()
