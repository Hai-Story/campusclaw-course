"""Lesson-six per-interface fixtures; requires the explicitly isolated test stack."""
import json
import os
import uuid
from urllib.request import Request, urlopen
from urllib.error import HTTPError


def fixture(c, token=None):
    secret = os.environ['APITEST_MCP_TOKEN']
    base = os.environ.get('APITEST_MCP_BASE_URL', 'http://mcp-demo:8090')
    name = 'campus-' + uuid.uuid4().hex[:12]
    server = c.expect('POST', '/api/mcp/servers', dict(name=name, base_url=base, token=secret), 201, token)
    assert server['has_token'] and 'token' not in server and secret not in json.dumps(server)
    assert c.scalar('SELECT name FROM mcp_servers WHERE id=%s', (server['id'],)) == name
    result = c.expect('POST', f"/api/mcp/servers/{server['id']}/connect", token=token)
    assert result['server']['status'] == 'connected'
    assert {'formula_hint','unit_convert'} <= {t['name'] for t in result['tools']}
    assert c.scalar('SELECT COUNT(*) FROM mcp_tools WHERE server_id=%s', (server['id'],)) >= 2
    return server


def source(server):
    return f"名称：MCP解题引导{uuid.uuid4().hex[:8]}\n适用条件：MCP\n工具：formula_hint@{server['name']},unit_convert@{server['name']}\n启用：是\n\n只给检查步骤和单位方向，不给整题最终答案。"


def common(c, method, path, payload=None):
    c.expect(method, path, payload, 401, '')
    c.expect(method, path, payload, 403, c.student)


def run_case(c, key):
    if key == 'POST /api/mcp/servers':
        payload = dict(name='registration-' + uuid.uuid4().hex[:8], base_url=os.environ.get('APITEST_MCP_BASE_URL','http://mcp-demo:8090'), token=os.environ['APITEST_MCP_TOKEN'])
        common(c, 'POST', '/api/mcp/servers', payload)
        server = c.expect('POST','/api/mcp/servers',payload,201)
        c.expect('POST','/api/mcp/servers',payload,400)
        assert c.scalar('SELECT status FROM mcp_servers WHERE id=%s',(server['id'],))=='disconnected'
        assert 'token' not in server
        return
    server = fixture(c)
    sid = server['id']; root=f'/api/mcp/servers/{sid}'
    method, template = key.split(' ',1); path=template.replace('{id}',str(sid))
    payload=dict(name=server['name'],base_url=server['base_url']) if method=='PUT' else None
    common(c,method,path,payload)
    if '{id}' in template:
        foreign=fixture(c,c.teacher_b)
        c.same404(method,template.replace('{id}',str(foreign['id'])),template.replace('{id}','999999999'),payload=payload)
    if key == 'GET /api/mcp/servers':
        foreign=fixture(c,c.teacher_b)
        result=c.expect(method,path)
        assert sid in [s['id'] for s in result['servers']] and foreign['id'] not in [s['id'] for s in result['servers']]
        assert os.environ['APITEST_MCP_TOKEN'] not in json.dumps(result)
    elif key == 'GET /api/mcp/servers/{id}':
        result=c.expect(method,path);assert result['id']==sid and result['has_token'] and 'token' not in result
    elif key == 'PUT /api/mcp/servers/{id}':
        before=c.scalar('SELECT token_text FROM mcp_servers WHERE id=%s',(sid,))
        result=c.expect(method,path,payload);assert result['status']=='disconnected' and result['has_token']
        assert c.scalar('SELECT token_text FROM mcp_servers WHERE id=%s',(sid,))==before
        c.expect('PUT',path,dict(payload,token=''));assert c.scalar('SELECT token_text FROM mcp_servers WHERE id=%s',(sid,))==''
    elif key == 'DELETE /api/mcp/servers/{id}':
        c.expect(method,path,status=204);assert c.scalar('SELECT COUNT(*) FROM mcp_servers WHERE id=%s',(sid,))==0
        assert c.scalar('SELECT COUNT(*) FROM mcp_tools WHERE server_id=%s',(sid,))==0
        assert c.scalar('SELECT COUNT(*) FROM mcp_tool_changes WHERE server_id IS NULL AND server_name=%s',(server['name'],))>=2
    elif key == 'POST /api/mcp/servers/{id}/connect':
        n=c.scalar('SELECT COUNT(*) FROM mcp_tools WHERE server_id=%s',(sid,));c.expect(method,path);assert c.scalar('SELECT COUNT(*) FROM mcp_tools WHERE server_id=%s',(sid,))==n
        c.expect('PUT',root,dict(name=server['name'],base_url=server['base_url'],token='wrong-test-token'))
        c.expect(method,path,status=503);assert c.scalar('SELECT status FROM mcp_servers WHERE id=%s',(sid,))=='disconnected'
    elif key == 'POST /api/mcp/servers/{id}/disconnect':
        c.expect(method,path);c.expect(method,path);assert c.scalar('SELECT COUNT(*) FROM mcp_tools WHERE server_id=%s',(sid,))==0
        assert c.expect('GET',root+'/tools')['tools']==[]
    elif key == 'GET /api/mcp/servers/{id}/tools':
        result=c.expect(method,path);assert all(t['inputSchema']['type']=='object' and t['description'] and t['hash'] for t in result['tools'])
    elif key == 'GET /api/mcp/servers/{id}/changes':
        result=c.expect(method,path);assert len(result['changes'])>=2
        c.expect('GET',path+'?limit=101',status=400)
    elif key == 'GET /api/mcp/audits':
        end_to_end(c, server)
    else:raise AssertionError(key)


def end_to_end(c, server=None):
    server=server or fixture(c);root=f"/api/mcp/servers/{server['id']}"
    raw=source(server);skill=c.expect('POST','/api/skills',{'source':raw},201)
    assert c.scalar('SELECT COUNT(*) FROM mcp_tool_approvals WHERE skill_id=%s',(skill['id'],))==2
    invalid=raw.replace('unit_convert@','unknown_tool@')
    c.expect('PUT',f"/api/skills/{skill['id']}",{'source':invalid},400)
    assert c.scalar('SELECT source_text FROM tutor_skills WHERE id=%s',(skill['id'],))==raw
    assert c.scalar('SELECT COUNT(*) FROM mcp_tool_approvals WHERE skill_id=%s',(skill['id'],))==2
    agent=c.expect('POST','/api/agents',dict(name='MCP助手'+c.suffix+uuid.uuid4().hex[:6],prompt='耐心地给出学习提示',enabled=True,skill_ids=[skill['id']],material_ids=[]),201)
    conv=c.expect('POST','/api/conversations',{'agent_id':agent['id']},201,c.student)
    def ask(question, statuses):
        events=c.stream(question,conversation=conv['id']);c.assert_saved(events)
        calls=[call for kind,data,_ in events if kind=='meta' for call in data.get('tool_calls',[])]
        assert [call['status'] for call in calls]==statuses,(question,calls)
        first_delta=next(i for i,e in enumerate(events) if e[0]=='delta')
        assert all(i<first_delta for i,e in enumerate(events) if e[0]=='meta')
        row=c.rows('SELECT tool_calls_json FROM tutor_messages WHERE id=%s',(events[-1][1]['message_id'],))[0]
        assert (json.loads(row['tool_calls_json']) if row['tool_calls_json'] else [])==calls
        return calls
    calls=ask('MCP双步骤：先去括号，再核对米到厘米的单位',['ok','ok']);assert [x['round'] for x in calls]==[1,2]
    ask('MCP不调用：直接给检查方向',[])
    ask('MCP参数错误：检查参数',['denied'])
    ask('MCP工具失败：试试不同量纲',['error'])
    c.expect('POST',root+'/disconnect')
    ask('MCP双步骤：断开后再试',['denied','denied'])
    audit=c.expect('GET','/api/mcp/audits')['audits'];ids={x['call_id'] for x in audit};assert all(x['call_id'] in ids for x in calls)
    records=c.rows('SELECT status FROM mcp_call_audits WHERE conversation_id=%s',(conv['id'],));assert {'ok','error','denied'}<={x['status'] for x in records}
    c.expect('GET','/api/mcp/audits',status=403,token=c.student)
    other=c.expect('GET','/api/mcp/audits',token=c.teacher_b)['audits'];assert not any(x['conversation_id']==conv['id'] for x in other)
    for status in ('ok','error','denied'):
        assert all(x['status']==status for x in c.expect('GET','/api/mcp/audits?status='+status)['audits'])
    c.expect('GET','/api/mcp/audits?limit=101',status=400)
    c.expect('DELETE',root,status=204)
    ask('MCP双步骤：已删除服务器',['denied','denied'])
    assert c.scalar('SELECT COUNT(*) FROM mcp_call_audits WHERE conversation_id=%s AND server_id IS NULL',(conv['id'],))>=2
    # Reload history retains the metadata under owner-only access.
    history=c.expect('GET',f"/api/conversations/{conv['id']}",token=c.student)
    assert any(m.get('tool_calls') for m in history['messages'] if m['role']=='assistant')
    c.same404('GET',f"/api/conversations/{conv['id']}",'/api/conversations/999999999',token=c.teacher)
