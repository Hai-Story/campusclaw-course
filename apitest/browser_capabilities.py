"""Real browser acceptance against the isolated app, not intercepted API fixtures."""
import os
import re
from pathlib import Path
os.environ.setdefault('PLAYWRIGHT_BROWSERS_PATH',str(Path(__file__).resolve().parents[1]/'.tools/playwright'))
from playwright.sync_api import sync_playwright, expect
from support import Context


def main():
    c=Context();reports=Path(__file__).parent/'reports';reports.mkdir(exist_ok=True)
    try:
        with sync_playwright() as p:
            browser=p.chromium.launch(headless=True,chromium_sandbox=True)
            context=browser.new_context(viewport={'width':1440,'height':1000})
            page=context.new_page();errors=[]
            page.on('pageerror',lambda e:errors.append(str(e)))
            def login(user):
                page.goto(c.base);page.wait_for_load_state('networkidle')
                page.get_by_label('账号',exact=True).fill(user)
                page.get_by_label('密码',exact=True).fill(c.password)
                page.get_by_role('button',name='登录',exact=True).click()
                expect(page.get_by_role('heading',name='教研材料',exact=True)).to_be_visible()
                page.wait_for_load_state('networkidle')
            login('teacher_a')
            page.locator('.rail').get_by_role('button',name=re.compile('助手配置')).click()
            page.wait_for_load_state('networkidle')
            # Built-in templates are read-only and copying produces an editable local template.
            templates=page.get_by_label('选择提示词模板',exact=True)
            templates.select_option(label='数学解题模板（内置）')
            template=page.get_by_label('模板正文',exact=True).input_value()
            expect(page.get_by_label('模板正文',exact=True)).to_have_attribute('readonly','')
            expect(page.get_by_role('button',name='保存模板',exact=True)).to_have_count(0)
            role=page.get_by_label('角色提示词',exact=True);role.fill('已有角色提示词')
            page.once('dialog',lambda d:d.dismiss())
            page.get_by_role('button',name='应用到角色提示词',exact=True).click()
            expect(role).to_have_value('已有角色提示词')
            page.once('dialog',lambda d:d.accept())
            page.get_by_role('button',name='应用到角色提示词',exact=True).click()
            expect(role).to_have_value(template)
            page.get_by_role('button',name='复制为自定义',exact=True).click()
            template_name='浏览器模板'+c.suffix
            page.get_by_label('模板名称',exact=True).fill(template_name)
            page.get_by_role('button',name='保存模板',exact=True).click()
            expect(page.get_by_role('status').filter(has_text='模板已保存')).to_be_visible()
            assert c.scalar('SELECT builtin FROM tutor_prompt_templates WHERE name=%s',(template_name,))==0
            agent_name='浏览器五项助手'+c.suffix
            page.get_by_label('助手名称',exact=True).fill(agent_name)
            page.get_by_role('button',name='保存助手',exact=True).click()
            expect(page.get_by_role('status').filter(has_text='助手配置已保存')).to_be_visible()
            agent_id=c.scalar('SELECT id FROM tutor_agents WHERE name=%s',(agent_name,))
            page.get_by_label('模板正文',exact=True).fill('模板后来改了，已有助手不应改变。')
            page.get_by_role('button',name='保存模板',exact=True).click()
            expect(page.get_by_role('status').filter(has_text='模板已保存')).to_be_visible()
            assert c.scalar('SELECT prompt FROM tutor_agents WHERE id=%s',(agent_id,))==template
            page.once('dialog',lambda d:d.accept())
            page.get_by_role('button',name='删除模板',exact=True).click()
            expect(page.get_by_role('status').filter(has_text='模板已删除')).to_be_visible()
            assert c.scalar('SELECT prompt FROM tutor_agents WHERE id=%s',(agent_id,))==template
            # Both file formats go through the real preview and save endpoints.
            file_input=page.get_by_label('导入 .txt / .md',exact=True)
            legacy=c.source.replace('名称：数学解题引导','名称：浏览器课程')
            file_input.set_input_files({'name':'course.txt','mimeType':'text/plain','buffer':legacy.encode()})
            expect(page.get_by_role('region',name='技能解析预览')).to_contain_text('浏览器课程'+c.suffix)
            page.get_by_role('button',name='保存技能',exact=True).click()
            expect(page.get_by_role('status').filter(has_text='技能已保存')).to_be_visible()
            skill_name='browser-standard-'+c.suffix
            raw='---\nname: '+skill_name+'\ndescription: 浏览器技能验收\nallowed-tools: shell\n---\n先检查步骤，结合已批准的工具给下一步提示。'
            file_input.set_input_files({'name':'SKILL.md','mimeType':'text/markdown','buffer':raw.encode()})
            expect(page.get_by_role('region',name='技能解析预览')).to_contain_text(skill_name)
            page.get_by_label('技能触发关键词',exact=True).fill('MCP')
            # Register a real local MCP service; successful save clears the password field.
            server_name='browser-'+c.suffix
            page.get_by_label('服务器引用名',exact=True).fill(server_name)
            page.get_by_label('MCP Base URL',exact=True).fill('http://mcp-demo:8090')
            page.get_by_label('MCP Bearer 令牌',exact=True).fill(os.environ['APITEST_MCP_TOKEN'])
            page.get_by_role('button',name='保存服务器',exact=True).click()
            expect(page.get_by_role('status').filter(has_text='服务器已保存')).to_be_visible()
            expect(page.get_by_label('MCP Bearer 令牌',exact=True)).to_have_value('')
            page.get_by_role('button',name='连接并发现',exact=True).click()
            expect(page.get_by_role('status').filter(has_text='连接成功')).to_be_visible()
            page.get_by_role('button',name='刷新可用工具声明',exact=True).click()
            page.get_by_label('技能工具引用',exact=True).fill('formula_hint@'+server_name+'，unit_convert@'+server_name)
            review=page.locator('.mcp-reference-review')
            expect(review).to_contain_text('formula_hint@'+server_name)
            expect(review).not_to_contain_text('未连接或未发现')
            page.get_by_role('button',name='保存技能',exact=True).click()
            expect(page.get_by_role('status').filter(has_text='技能已保存')).to_be_visible()
            skill_id=c.scalar('SELECT id FROM tutor_skills WHERE name=%s',(skill_name,))
            assert c.scalar('SELECT source_text FROM tutor_skills WHERE id=%s',(skill_id,))==raw
            assert c.scalar('SELECT COUNT(*) FROM mcp_tool_approvals WHERE skill_id=%s',(skill_id,))==2
            page.get_by_label(skill_name,exact=True).check()
            page.get_by_role('button',name='保存助手',exact=True).click()
            expect(page.get_by_role('status').filter(has_text='助手配置已保存')).to_be_visible()
            page.get_by_role('button',name='退出登录',exact=True).click()
            login('student_a1')
            expect(page.locator('.rail').get_by_role('button',name=re.compile('助手配置'))).to_have_count(0)
            page.locator('.rail').get_by_role('button',name=re.compile('解题会话')).click()
            expect(page.get_by_role('button',name='使用数学解题助手',exact=True)).to_be_visible()
            assert page.locator('.application-card strong').all_text_contents()[:3]==['数学解题助手','语文阅读助手','英语学习助手']
            page.screenshot(path=str(reports/'builtin-apps.png'))
            page.get_by_role('button',name='使用数学解题助手',exact=True).click()
            expect(page.get_by_role('heading',name='新的提问',exact=True)).to_be_visible()
            page.get_by_label('你的问题',exact=True).fill('方程去括号时应该检查什么？')
            page.get_by_role('button',name='发送问题',exact=True).click()
            expect(page.get_by_role('button',name='发送问题',exact=True)).to_be_visible(timeout=15000)
            expect(page.locator('.tutor-message.assistant .markdown-body').last).not_to_be_empty()
            page.get_by_label('选择智能体',exact=True).select_option(str(agent_id))
            page.get_by_role('button',name='新建会话',exact=True).click()
            expect(page.get_by_role('heading',name='新的提问',exact=True)).to_be_visible()
            question='MCP双步骤：先去括号，再核对米到厘米的单位'
            page.get_by_label('你的问题',exact=True).fill(question)
            page.get_by_role('button',name='发送问题',exact=True).click()
            expect(page.get_by_role('button',name='发送问题',exact=True)).to_be_visible(timeout=15000)
            expect(page.locator('.tool-call-list .tool-call')).to_have_count(2)
            expect(page.locator('.tool-call-list')).to_contain_text('成功')
            page.screenshot(path=str(reports/'standard-skill-mcp.png'))
            page.reload();page.wait_for_load_state('networkidle')
            page.locator('.rail').get_by_role('button',name=re.compile('解题会话')).click()
            page.locator('.conversation-list button').filter(has_text=question[:24]).first.click()
            expect(page.locator('.tool-call-list .tool-call')).to_have_count(2)
            page.set_viewport_size({'width':375,'height':812})
            page.get_by_role('button',name='切换主题',exact=True).click()
            assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'),'mobile overflow'
            page.screenshot(path=str(reports/'builtin-mobile-dark.png'))
            assert not errors,errors
            browser.close()
            print('PASS browser: templates/copy/confirmation/snapshot, both skill formats, MCP discovery/approval/two rounds/replay, apps, mobile and dark theme')
    finally:c.close()

if __name__=='__main__':main()
