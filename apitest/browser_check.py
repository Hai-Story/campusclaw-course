"""Headless browser acceptance against the same isolated fixtures as the API suite."""
import re
import json
import os
from pathlib import Path
os.environ.setdefault("PLAYWRIGHT_BROWSERS_PATH",str(Path(__file__).resolve().parents[1]/".tools/playwright"))
from playwright.sync_api import sync_playwright, expect
from support import Context


def main():
    c=Context();root=Path(__file__).resolve().parent/"reports"
    with sync_playwright() as p:
        browser=p.chromium.launch(headless=True)
        context=browser.new_context(viewport={"width":1440,"height":1000})
        page=context.new_page();errors=[]
        page.on("pageerror",lambda error:errors.append(str(error)))
        def login(name):
            page.goto(c.base);page.wait_for_load_state("networkidle")
            page.get_by_label("账号",exact=True).fill(name);page.get_by_label("密码",exact=True).fill(c.password)
            page.get_by_role("button",name="登录",exact=True).click()
            expect(page.get_by_role("heading",name="教研材料",exact=True)).to_be_visible()
            page.wait_for_load_state("networkidle")
        login("teacher_a")
        print("Rendered navigation:",page.locator(".rail nav button").all_text_contents())
        page.get_by_role("button",name=re.compile("助手配置")).click();page.wait_for_load_state("networkidle")
        expect(page.get_by_role("heading",name="智能体配置",exact=True)).to_be_visible()
        page.get_by_label("助手名称",exact=True).fill("浏览器助手"+c.suffix)
        page.get_by_label("角色提示词",exact=True).fill("你是耐心的中文学习助手。")
        page.get_by_label(c.skill['name'],exact=True).check()
        title=c.scalar('SELECT title FROM materials WHERE id=%s',(c.material,))
        page.get_by_label(title,exact=True).check()
        page.get_by_role("button",name="保存助手",exact=True).click()
        expect(page.get_by_role("status").filter(has_text="助手配置已保存")).to_be_visible()
        agent=c.rows('SELECT id,prompt FROM tutor_agents WHERE name=%s',('浏览器助手'+c.suffix,))[0]
        assert agent['prompt']=='你是耐心的中文学习助手。'
        page.get_by_label('助手名称',exact=True).fill(c.agent['name'])
        page.get_by_role('button',name='保存助手',exact=True).click()
        expect(page.get_by_role('alert').filter(has_text='本班名称已存在')).to_be_visible()
        page.get_by_label('助手名称',exact=True).fill('浏览器助手'+c.suffix)
        enabled=page.get_by_label('启用助手（停用只阻止新建会话）',exact=True)
        for state in [False,True]:
            enabled.set_checked(state)
            page.get_by_role('button',name='保存助手',exact=True).click()
            expect(page.get_by_role('status').filter(has_text='助手配置已保存')).to_be_visible()
            assert bool(c.scalar('SELECT enabled FROM tutor_agents WHERE id=%s',(agent['id'],)))==state
        # File import uses real UTF-8 bytes and the course parser.
        imported=c.source.replace('名称：数学解题引导','名称：文件解题引导')
        page.get_by_label("导入 .txt / .md",exact=True).set_input_files({"name":"browser-skill.txt","mimeType":"text/plain","buffer":imported.encode()})
        expect(page.get_by_label("技能原文",exact=True)).to_have_value(imported)
        page.get_by_role("button",name="保存技能",exact=True).click()
        expect(page.get_by_role("status").filter(has_text="技能已保存")).to_be_visible()
        assert c.scalar('SELECT COUNT(*) FROM tutor_skills WHERE source_text=%s',(imported,))==1
        page.evaluate('window.scrollTo(0,0)')
        page.screenshot(path=str(root/'teacher-settings.png'),full_page=True)
        for state in ['否','是']:
            page.get_by_label('技能原文',exact=True).fill(imported.replace('启用：是','启用：'+state))
            page.get_by_role('button',name='保存技能',exact=True).click()
            expect(page.get_by_role('status').filter(has_text='技能已保存')).to_be_visible()
            assert bool(c.scalar('SELECT enabled FROM tutor_skills WHERE name=%s',('文件解题引导'+c.suffix,)))==(state=='是')
        page.once('dialog',lambda dialog:dialog.accept())
        page.get_by_role('button',name='删除技能',exact=True).click()
        expect(page.get_by_role('status').filter(has_text='技能已删除')).to_be_visible()
        assert c.scalar('SELECT COUNT(*) FROM tutor_skills WHERE name=%s',('文件解题引导'+c.suffix,))==0
        page.get_by_role("button",name="退出登录",exact=True).click()
        login("student_a1")
        expect(page.get_by_role("button",name=re.compile("助手配置"))).to_have_count(0)
        page.get_by_role("button",name=re.compile("解题会话")).click();page.wait_for_load_state("networkidle")
        page.get_by_label("选择智能体",exact=True).select_option(str(agent['id']))
        page.get_by_role("button",name="新建会话",exact=True).click()
        expect(page.get_by_role("heading",name="新的提问",exact=True)).to_be_visible()
        page.get_by_label("你的问题",exact=True).fill("方程去括号 mock-slow：我的这一步哪里错了？")
        page.get_by_role("button",name="发送问题",exact=True).click()
        expect(page.get_by_role("button",name="停止生成",exact=True)).to_be_visible()
        expect(page.locator('.tutor-citations button').first).to_be_visible()
        partial=page.locator('.tutor-message.assistant .markdown-body').last.inner_text()
        expect(page.get_by_role("button",name="发送问题",exact=True)).to_be_visible(timeout=10000)
        final=page.locator('.tutor-message.assistant .markdown-body').last.inner_text()
        assert len(final)>len(partial) and 'x=2' not in final
        history_title=page.locator('.conversation-heading h2').inner_text()
        page.evaluate('window.scrollTo(0,0)')
        page.screenshot(path=str(root/'student-chat.png'),full_page=True)
        page.locator('.tutor-citations button').first.click()
        expect(page.get_by_role('heading',name=title,exact=True)).to_be_visible()
        assert page.evaluate('window.lesson5Xss !== true')
        page.get_by_role('button',name=re.compile('关闭')).first.click()
        page.get_by_label("你的问题",exact=True).fill("继续解释这一步的理由")
        page.get_by_role("button",name="发送问题",exact=True).click()
        expect(page.get_by_role("button",name="发送问题",exact=True)).to_be_visible(timeout=10000)
        assert page.locator('.tutor-message.user').count()==2
        page.reload();page.wait_for_load_state('networkidle')
        page.get_by_role("button",name=re.compile("解题会话")).click();page.wait_for_load_state('networkidle')
        page.locator('.conversation-list button').filter(has_text=history_title).first.click()
        expect(page.locator('.tutor-message.user')).to_have_count(2)
        page.get_by_label("你的问题",exact=True).fill("方程 mock-slow 再试一次")
        page.get_by_role("button",name="发送问题",exact=True).click()
        expect(page.get_by_role("button",name="停止生成",exact=True)).to_be_visible()
        expect(page.locator('.tutor-message.assistant .markdown-body').last).to_contain_text('你在')
        page.get_by_role("button",name="停止生成",exact=True).click()
        expect(page.get_by_role('alert').filter(has_text='已停止生成')).to_be_visible()
        # Mobile layout, dark theme, and no horizontal overflow.
        page.set_viewport_size({'width':375,'height':812})
        page.get_by_role('button',name='切换主题',exact=True).click()
        assert page.evaluate('document.documentElement.dataset.theme')=='dark'
        assert page.evaluate('document.documentElement.scrollWidth <= window.innerWidth'), 'mobile horizontal overflow'
        page.evaluate('window.scrollTo(0,0)')
        page.screenshot(path=str(root/'student-mobile-dark.png'),full_page=True)
        c.expect('DELETE',f'/api/materials/{c.material}',status=204)
        page.locator('.tutor-citations button').first.click()
        expect(page.locator('.toast-region').filter(has_text='未找到材料')).to_be_visible()
        page.get_by_role('button',name='知识检索',exact=True).click()
        expect(page.get_by_role('heading',name='可追溯检索',exact=True)).to_be_visible()
        page.get_by_role('button',name='知识问答',exact=True).click()
        expect(page.get_by_role('heading',name='依据资料问答',exact=True)).to_be_visible()
        page.get_by_role('button',name='材料库',exact=True).click()
        expect(page.get_by_role('heading',name='教研材料',exact=True)).to_be_visible()
        # Swagger is publicly available; authorize using a token obtained by the test login API.
        swagger=context.new_page();swagger.on('pageerror',lambda error:errors.append(str(error)));swagger.goto(c.base+'/swagger/');swagger.wait_for_load_state('networkidle')
        expect(swagger.locator('.swagger-ui')).to_be_visible()
        assert not swagger.locator('.errors-wrapper').is_visible()
        swagger.screenshot(path=str(root/'swagger.png'),full_page=True)
        login_block=swagger.locator('#operations-default-login')
        login_block.locator('.opblock-summary').click()
        login_block.get_by_role('button',name='Try it out',exact=True).click()
        login_block.locator('textarea').fill(json.dumps({'username':'student_a1','password':c.password}))
        login_block.get_by_role('button',name='Execute',exact=True).click()
        expect(login_block.locator('.live-responses-table .response-col_status').filter(has_text='200').first).to_be_visible()
        login_block.locator('.opblock-summary').click()
        swagger.get_by_role('button',name=re.compile('Authorize')).first.click()
        swagger.locator('.auth-container input').fill(c.student)
        swagger.locator('.dialog-ux').get_by_role('button',name='Apply credentials',exact=True).click()
        swagger.get_by_role('button',name='Close',exact=True).click()
        for operation in ['get_api_agents','get_api_skills','get_api_conversations']:
            block=swagger.locator('#operations-default-'+operation)
            block.locator('.opblock-summary').click()
            block.get_by_role('button',name='Try it out',exact=True).click()
            block.get_by_role('button',name='Execute',exact=True).click()
            expect(block.locator('.live-responses-table .response-col_status').filter(has_text='200').first).to_be_visible()
        search=swagger.locator('#operations-default-get_api_knowledge_search')
        search.locator('.opblock-summary').click()
        search.get_by_role('button',name='Try it out',exact=True).click()
        search.locator('input[placeholder="q"]').fill('方程')
        search.get_by_role('button',name='Execute',exact=True).click()
        expect(search.locator('.live-responses-table .response-col_status').filter(has_text='200').first).to_be_visible()
        create=swagger.locator('#operations-default-post_api_conversations')
        create.locator('.opblock-summary').click()
        create.get_by_role('button',name='Try it out',exact=True).click()
        create.locator('textarea').fill(json.dumps({'agent_id':agent['id']}))
        create.get_by_role('button',name='Execute',exact=True).click()
        expect(create.locator('.live-responses-table .response-col_status').filter(has_text='201').first).to_be_visible()
        assert not errors, 'JavaScript page errors'
        browser.close()
    c.close();print('PASS browser: teacher edits/imports, student streams/follows citations/restores/cancels, mobile themes, old navigation, Swagger Bearer requests')

if __name__=='__main__':main()
