"""Standard SKILL.md against real MCP and a controllable proxy, with mock model decisions."""
import json
import os
from urllib.request import Request,urlopen
from support import Context
from mcp_support import fixture


def control(mode='normal',revision=1):
    request=Request('http://127.0.0.1:18768/control',data=json.dumps(dict(mode=mode,revision=revision)).encode(),headers={'Content-Type':'application/json','Authorization':'Bearer '+os.environ['APITEST_MCP_TOKEN']})
    with urlopen(request,timeout=5) as response:assert response.status==200


def run(c):
    secret=os.environ['APITEST_MCP_TOKEN'];control()
    server=c.expect('POST','/api/mcp/servers',dict(name='standard-'+c.suffix,base_url='http://mcp-fixture:8091',token=secret),201)
    root=f"/api/mcp/servers/{server['id']}"
    try:
        c.expect('POST',root+'/connect')
        raw='---\nname: standard-mcp-'+c.suffix+'\ndescription: 数学步骤提示\nallowed-tools: shell\n---\n先检查步骤，必要时使用已批准的只读工具。'
        routing=dict(trigger='MCP',subject='',tools=['formula_hint@'+server['name'],'unit_convert@'+server['name']],enabled=True)
        skill=c.expect('POST','/api/skills',dict(source=raw,routing=routing),201)
        assert c.scalar('SELECT COUNT(*) FROM mcp_tool_approvals WHERE skill_id=%s',(skill['id'],))==2
        agent=c.expect('POST','/api/agents',dict(name='标准工具助手'+c.suffix,prompt='耐心解释学习步骤',enabled=True,skill_ids=[skill['id']],material_ids=[]),201)
        conv=c.expect('POST','/api/conversations',{'agent_id':agent['id']},201,c.student)
        def ask(statuses):
            events=c.stream('MCP双步骤：先去括号，再检查米到厘米的单位',conversation=conv['id']);c.assert_saved(events)
            calls=[call for kind,data,_ in events if kind=='meta' for call in data.get('tool_calls',[])]
            assert [x['status'] for x in calls]==statuses,(statuses,[x['status'] for x in calls])
            first_delta=next(i for i,e in enumerate(events) if e[0]=='delta')
            assert all(i<first_delta for i,e in enumerate(events) if e[0]=='meta')
            saved=c.expect('GET',f"/api/conversations/{conv['id']}",token=c.student)['messages'][-1].get('tool_calls',[])
            assert saved==calls
            assert secret not in json.dumps(calls)
            return calls
        calls=ask(['ok','ok']);assert [x['round'] for x in calls]==[1,2]
        control(revision=2);c.expect('POST',root+'/connect')
        ask(['denied','denied'])
        c.expect('PUT',f"/api/skills/{skill['id']}",dict(source=raw,routing=routing))
        ask(['ok','ok'])
        control(mode='timeout',revision=2)
        ask(['error','error'])
        control(revision=2)
        c.expect('POST',root+'/disconnect')
        ask(['denied','denied'])
        c.expect('POST',root+'/connect')
        disabled=dict(routing,enabled=False)
        c.expect('PUT',f"/api/skills/{skill['id']}",dict(source=raw,routing=disabled))
        ask([])
        c.expect('PUT',f"/api/skills/{skill['id']}",dict(source=raw,routing=routing),403,c.student)
        foreign=fixture(c,c.teacher_b)
        c.expect('PUT',f"/api/skills/{skill['id']}",dict(source=raw,routing=dict(routing,tools=['formula_hint@'+foreign['name']])),400)
        audits=c.expect('GET','/api/mcp/audits?server_id='+str(server['id']))['audits']
        assert {'ok','error','denied'}<={x['status'] for x in audits}
        assert c.scalar("SELECT COUNT(*) FROM mcp_call_audits WHERE server_id=%s AND status='pending'",(server['id'],))==0
        print('PASS standard MCP: two rounds, approval changes, reapproval, timeout, disconnect, disable, permissions and audits')
    finally:
        control()
        c.expect('DELETE',root,status=204)

if __name__=='__main__':
    c=Context()
    try:run(c)
    finally:c.close()
