"""Offline UI verification with explicit API fixtures; does not claim DB/end-to-end acceptance."""
import json
import os
import re
from pathlib import Path
os.environ.setdefault('PLAYWRIGHT_BROWSERS_PATH',str(Path(__file__).resolve().parents[1]/'.tools/playwright'))
from playwright.sync_api import sync_playwright, expect


def main():
    base=os.environ.get('MCP_UI_BASE_URL','http://127.0.0.1:15176')
    reports=Path(__file__).resolve().parent/'reports';reports.mkdir(exist_ok=True)
    with sync_playwright() as p:
        browser=p.chromium.launch(headless=True)
        context=browser.new_context(viewport={'width':1440,'height':1000})
        page=context.new_page();errors=[];page.on('pageerror',lambda e:errors.append(str(e)))
        state={'role':'teacher','servers':[],'skills':[],'connected':False,'conversation':False,'saved':[]}
        tool={'name':'formula_hint','description':'只给学习提示，不给最终得数。<script>window.mcpXss=true</script>','inputSchema':{'type':'object','properties':{'query':{'type':'string'}},'required':['query']},'hash':'a'*64}
        call={'ref':'formula_hint@campus-tools','status':'ok','text':'先检查每一项 <script>window.mcpXss=true</script>','round':1,'call_id':'ui-call-1'}
        def response(route,payload,status=200):route.fulfill(status=status,content_type='application/json',body=json.dumps(payload,ensure_ascii=False))
        def api(route):
            request=route.request;path=request.url.split('/api',1)[1].split('?',1)[0];method=request.method;data=request.post_data_json if request.post_data else {}
            if path=='/login':
                state['role']='teacher' if data['username']=='teacher_a' else 'student'
                return response(route,{'user':{'id':1,'role':state['role'],'username':data['username'],'class_id':1,'class_name':'A班'},'token':'ui-fixture-only'})
            if path=='/logout':return response(route,{},204)
            if path=='/me':return response(route,{'user':{'id':1,'role':state['role'],'username':'fixture','class_id':1,'class_name':'A班'}})
            if path=='/materials':return response(route,{'materials':[]})
            if path=='/agents':return response(route,{'agents':[{'id':1,'class_id':1,'name':'学习助手','prompt':'','enabled':True,'skill_ids':[1],'material_ids':[]}]})
            if path=='/skills':
                if method=='POST':
                    if 'unknown@' in data['source']:return response(route,{'error':'工具引用不可用'},400)
                    skill={'id':1,'name':'MCP解题引导','source':data['source'],'enabled':True,'tools':['formula_hint@campus-tools'],'trigger':'MCP','subject':'','description':'引导'};state['skills']=[skill];return response(route,skill,201)
                return response(route,{'skills':state['skills']})
            if path=='/mcp/servers':
                if method=='POST':
                    server={'id':1,'name':data['name'],'base_url':data['base_url'],'has_token':True,'status':'disconnected','revision':0,'protocol_version':'','last_error':''};state['servers']=[server];return response(route,server,201)
                return response(route,{'servers':state['servers']})
            if path.endswith('/connect'):state['connected']=True;state['servers'][0]['status']='connected';return response(route,{'server':state['servers'][0],'tools':[tool]})
            if path.endswith('/disconnect'):state['connected']=False;state['servers'][0]['status']='disconnected';return response(route,state['servers'][0])
            if path.endswith('/tools'):return response(route,{'tools':[tool] if state['connected'] else []})
            if path.endswith('/changes'):return response(route,{'changes':[],'next_before':0})
            if path=='/mcp/audits':return response(route,{'audits':[], 'next_before':''})
            if path=='/conversations':
                if method=='POST':state['conversation']=True;return response(route,{'id':1,'agent_id':1,'title':'新的提问','created_at':'2026-10-21T00:00:00Z','updated_at':'2026-10-21T00:00:00Z'},201)
                return response(route,{'conversations':[{'id':1,'agent_id':1,'title':'新的提问','created_at':'2026-10-21T00:00:00Z','updated_at':'2026-10-21T00:00:00Z'}] if state['conversation'] else []})
            if path=='/conversations/1':return response(route,{'conversation':{'id':1,'agent_id':1,'title':'新的提问'},'messages':state['saved']})
            if path=='/conversations/1/messages':
                calls=[call,{**call,'call_id':'ui-call-2','round':2,'status':'denied','text':'已断开，拒绝调用'}]
                frames=[('meta',{'status':'ok','skill':{'id':1,'name':'MCP解题引导'},'citations':[]}),('meta',{'tool_calls':[calls[0]]}),('meta',{'tool_calls':[calls[1]]}),('delta',{'text':'先自行核对每一项。'}),('done',{'message_id':2})]
                state['saved']=[{'id':1,'role':'user','content':data['content'],'citations':[],'skill_name':'','status':'complete','created_at':''},{'id':2,'role':'assistant','content':'先自行核对每一项。','citations':[],'skill_name':'MCP解题引导','status':'complete','created_at':'','tool_calls':calls}]
                return route.fulfill(status=200,headers={'Content-Type':'text/event-stream'},body=''.join('event: '+event+'\ndata: '+json.dumps(value,ensure_ascii=False)+'\n\n' for event,value in frames))
            return response(route,{'error':'fixture endpoint missing '+path},404)
        page.route('**/api/**',api)
        def login(user):
            page.goto(base);page.wait_for_load_state('networkidle')
            page.get_by_label('账号',exact=True).fill(user);page.get_by_label('密码',exact=True).fill('ui-fixture-password')
            page.get_by_role('button',name='登录',exact=True).click();expect(page.get_by_role('heading',name='教研材料',exact=True)).to_be_visible()
        login('teacher_a')
        # Discover available controls after the page has settled.
        page.get_by_role('button',name=re.compile('助手配置')).click();page.wait_for_load_state('networkidle')
        expect(page.get_by_role('heading',name='本班工具服务')).to_be_visible()
        page.get_by_label('MCP Bearer 令牌',exact=True).fill('ui-only-test-token')
        page.get_by_role('button',name='保存服务器',exact=True).click()
        expect(page.get_by_label('MCP Bearer 令牌',exact=True)).to_have_value('')
        page.get_by_role('button',name='连接并发现',exact=True).click()
        expect(page.get_by_role('status').filter(has_text='连接成功')).to_be_visible()
        assert page.evaluate('window.mcpXss !== true')
        source='名称：MCP解题引导\n适用条件：MCP\n工具：formula_hint@campus-tools\n启用：是\n\n只给提示。'
        page.get_by_label('技能原文',exact=True).fill(source)
        page.get_by_role('button',name='刷新可用工具声明',exact=True).click()
        expect(page.locator('.mcp-reference-review pre')).to_contain_text('query')
        page.get_by_role('button',name='保存技能',exact=True).click();expect(page.get_by_role('status').filter(has_text='技能已保存')).to_be_visible()
        assert page.evaluate("!JSON.stringify(localStorage).includes('ui-only-test-token') && !JSON.stringify(sessionStorage).includes('ui-only-test-token')")
        page.screenshot(path=str(reports/'mcp-settings-ui.png'),full_page=True)
        page.get_by_role('button',name='断开',exact=True).click();expect(page.get_by_role('status').filter(has_text='已断开')).to_be_visible()
        page.get_by_role('button',name='退出登录',exact=True).click();login('student_a1')
        expect(page.get_by_role('button',name=re.compile('助手配置'))).to_have_count(0)
        page.get_by_role('button',name=re.compile('解题会话')).click();page.wait_for_load_state('networkidle')
        page.get_by_label('选择智能体',exact=True).select_option('1');page.get_by_role('button',name='新建会话',exact=True).click()
        page.get_by_label('你的问题',exact=True).fill('MCP检查我的步骤');page.get_by_role('button',name='发送问题',exact=True).click()
        expect(page.locator('.tool-call-list .tool-call')).to_have_count(2)
        expect(page.locator('.tool-call-list')).to_contain_text('被拒')
        expect(page.locator('.tutor-message.assistant .markdown-body')).to_contain_text('先自行核对')
        assert page.evaluate('window.mcpXss !== true')
        for width in (1440,375):
            page.set_viewport_size({'width':width,'height':900})
            for theme in ('light','dark'):
                page.evaluate('(theme)=>document.documentElement.dataset.theme=theme',theme)
                assert page.evaluate('document.documentElement.scrollWidth <= window.innerWidth'),(width,theme)
                page.screenshot(path=str(reports/f'mcp-chat-ui-{width}-{theme}.png'),full_page=True)
        assert not errors,errors
        browser.close()
    print('PASS offline UI fixture: teacher MCP management, token clearing, skill declaration review, student controls, repeated meta, XSS, 375px/light/dark')

if __name__=='__main__':main()
