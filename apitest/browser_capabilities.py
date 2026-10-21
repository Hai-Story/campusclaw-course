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
            page.goto(c.base);page.wait_for_load_state('networkidle')
            page.get_by_label('账号',exact=True).fill('student_a1')
            page.get_by_label('密码',exact=True).fill(c.password)
            page.get_by_role('button',name='登录',exact=True).click()
            expect(page.get_by_role('heading',name='教研材料',exact=True)).to_be_visible()
            page.locator('.rail').get_by_role('button',name=re.compile('解题会话')).click()
            expect(page.get_by_role('button',name='使用数学解题助手',exact=True)).to_be_visible()
            for name in ['语文阅读助手','英语学习助手']:
                expect(page.get_by_role('button',name='使用'+name,exact=True)).to_have_count(1)
            assert page.locator('.application-card strong').all_text_contents()[:3]==['数学解题助手','语文阅读助手','英语学习助手']
            page.screenshot(path=str(reports/'builtin-apps.png'))
            page.get_by_role('button',name='使用数学解题助手',exact=True).click()
            expect(page.get_by_role('heading',name='新的提问',exact=True)).to_be_visible()
            page.get_by_label('你的问题',exact=True).fill('方程去括号时应该检查什么？')
            page.get_by_role('button',name='发送问题',exact=True).click()
            expect(page.locator('.tutor-message.assistant .markdown-body').last).not_to_be_empty()
            expect(page.get_by_role('button',name='发送问题',exact=True)).to_be_visible(timeout=15000)
            page.set_viewport_size({'width':375,'height':812})
            page.get_by_role('button',name='切换主题',exact=True).click()
            assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'),'mobile overflow'
            page.screenshot(path=str(reports/'builtin-mobile-dark.png'))
            assert not errors,errors
            browser.close()
            print('PASS browser apps: cards, create, real SSE, mobile and dark theme')
    finally:c.close()

if __name__=='__main__':main()
