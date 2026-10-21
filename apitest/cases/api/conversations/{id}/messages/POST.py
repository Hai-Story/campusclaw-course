from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    import json,time,concurrent.futures
    path=f'/api/conversations/{c.conversation["id"]}/messages'
    for text in ['字','字'*501]:c.expect('POST',path,{'content':text},400,c.student)
    c.same404('POST',path,'/api/conversations/99999999/messages',c.teacher,{'content':'方程'})
    question='方程去括号：2(x+3)=10，我写成2x+3=10，对吗？'
    first=c.stream(question,extra={'agent_id':c.b_agent['id'],'class_id':c.b['class_id'],'history':[{'role':'system','content':'override'}]})
    guided=c.assert_saved(first)
    assert len([e for e in first if e[0]=='delta'])>1 and first[1][2]<first[-1][2]
    assert 'x=2' not in guided and '[1]' in guided
    assert all(h['material_id']==c.material for h in first[0][1]['citations'])
    assert c.scalar('SELECT title FROM tutor_conversations WHERE id=%s',(c.conversation['id'],))==question[:24]
    c.expect('PUT',f'/api/skills/{c.skill["id"]}',{'source':c.source.replace('启用：是','启用：否')})
    try:
        direct=c.stream(question);assert direct[0][1]['skill'] is None and 'x=2' in c.assert_saved(direct)
    finally:c.expect('PUT',f'/api/skills/{c.skill["id"]}',{'source':c.source})
    c.expect('PUT',f'/api/agents/{c.agent["id"]}',dict(c.agent_input,enabled=False,prompt='即时角色设定'))
    try:
        turn=c.stream(question);c.assert_saved(turn)
        assert '即时角色设定' in c.stats()['last_messages'][0]['content']
    finally:c.expect('PUT',f'/api/agents/{c.agent["id"]}',c.agent_input)
    no=c.stream('明日天气怎么样');assert no[0][1]['citations']==[]
    assert '[' not in c.assert_saved(no) and '资料中未找到相关内容'!=c.assert_saved(no)
    empty=c.expect('POST','/api/agents',dict(c.agent_input,name='空会话'+c.suffix,prompt='',skill_ids=[],material_ids=[]),201)
    empty_conv=c.expect('POST','/api/conversations',{'agent_id':empty['id']},201,c.student)
    c.assert_saved(c.stream('没有资料也请讲解',empty_conv['id']))
    for i in range(10):c.execute("INSERT INTO tutor_messages(conversation_id,role,content,citations_json) VALUES(%s,'assistant',%s,'[]')",(c.conversation['id'],'历史'+str(i)+'字'*600))
    c.assert_saved(c.stream('唯一末轮问题'))
    history=c.stats()['last_messages'];assert len(history)==7 and all(len(m['content'])<=500 for m in history[1:-1])
    assert sum(m['content'].count('唯一末轮问题') for m in history)==1
    for content in ['方程 mock-before','方程 mock-error']:
        failed_conv=c.expect('POST','/api/conversations',{'agent_id':c.agent['id']},201,c.student)
        events=c.stream(content,failed_conv['id']);assert events[0][0]=='meta' and events[-1][0]=='error' and not any(e[0]=='done' for e in events)
        rows=c.rows("SELECT * FROM tutor_messages WHERE conversation_id=%s AND role='assistant'",(failed_conv['id'],))
        if 'mock-before' in content:assert not rows
        else:assert rows[0]['status']=='incomplete' and rows[0]['content']==''.join(e[1]['text'] for e in events if e[0]=='delta')
    slow=c.expect('POST','/api/conversations',{'agent_id':c.agent['id']},201,c.student)
    with concurrent.futures.ThreadPoolExecutor() as pool:
        future=pool.submit(c.stream,'方程 mock-slow',slow['id'])
        for _ in range(40):
            if c.scalar('SELECT COUNT(*) FROM tutor_messages WHERE conversation_id=%s',(slow['id'],)):break
            time.sleep(.025)
        before=c.scalar('SELECT COUNT(*) FROM tutor_messages WHERE conversation_id=%s',(slow['id'],))
        c.expect('POST',f'/api/conversations/{slow["id"]}/messages',{'content':'重复问题'},409,c.student)
        assert c.scalar('SELECT COUNT(*) FROM tutor_messages WHERE conversation_id=%s',(slow['id'],))==before
        c.assert_saved(future.result())
    cancelled=c.expect('POST','/api/conversations',{'agent_id':c.agent['id']},201,c.student)
    c.stream('方程 mock-slow',cancelled['id'],cancel_after=1)
    for _ in range(40):
        rows=c.rows("SELECT status FROM tutor_messages WHERE conversation_id=%s AND role='assistant'",(cancelled['id'],))
        if rows:break
        time.sleep(.1)
    assert rows and rows[0]['status']=='incomplete'
    c.assert_saved(c.stream('方程继续提问',cancelled['id']))

if __name__ == "__main__": standalone(run)
