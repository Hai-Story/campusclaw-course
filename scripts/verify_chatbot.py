"""Browser acceptance checks with mock API responses; no real credentials or data.

Run against the frontend dev server with a project-local Playwright installation:
  .venv/bin/python scripts/verify_chatbot.py
Optional: CHATBOT_BASE_URL and PLAYWRIGHT_CHROMIUM_EXECUTABLE.
"""

import json
import os
from pathlib import Path
from urllib.parse import parse_qs, urlparse

from playwright.sync_api import Error, expect, sync_playwright


BASE_URL = os.environ.get("CHATBOT_BASE_URL", "http://127.0.0.1:5178")
OUTPUT = Path(__file__).resolve().parents[1] / "output" / "playwright"
USER = {"id": 1, "username": "student_a1", "role": "student", "class_id": 1, "class_name": "A 班"}
MATERIAL = {"id": 7, "title": "文本细读教研笔记", "original_name": "note.md", "class_id": 1,
            "class_name": "A 班", "created_at": "2026-10-01T00:00:00Z", "size_bytes": 36,
            "media_type": "text/markdown", "content": "文本细读可以通过圈画关键词理解文章。"}
HIT = {"chunk_id": 42, "material_id": 7, "title": MATERIAL["title"], "original_name": "note.md",
       "chunk_index": 1, "excerpt": MATERIAL["content"], "start_offset": 0,
       "end_offset": len(MATERIAL["content"]), "offset_basis": "original", "rank": 1}


def run():
    OUTPUT.mkdir(parents=True, exist_ok=True)
    with sync_playwright() as p:
        options = {"headless": True}
        if os.environ.get("PLAYWRIGHT_CHROMIUM_EXECUTABLE"):
            options["executable_path"] = os.environ["PLAYWRIGHT_CHROMIUM_EXECUTABLE"]
        browser = p.chromium.launch(**options)
        context = browser.new_context(viewport={"width": 1440, "height": 1000})
        page = context.new_page()
        page_errors = []
        page.on("pageerror", lambda error: page_errors.append(str(error)))
        asks, searches, material_queries, delayed = [], [], [], []
        attempts = {}

        def fulfill(route, body, status=200):
            route.fulfill(status=status, content_type="application/json", body=json.dumps(body, ensure_ascii=False))

        def handle(route):
            request = route.request
            path = urlparse(request.url).path
            if path == "/api/login":
                fulfill(route, {"user": USER, "token": "frontend-ui-test-token"})
                return
            assert request.headers.get("authorization") == "Bearer frontend-ui-test-token"
            if path == "/api/me":
                fulfill(route, {"user": USER})
            elif path == "/api/logout":
                route.fulfill(status=204)
            elif path == "/api/materials":
                material_queries.append(parse_qs(urlparse(request.url).query).get("q", [""])[0])
                fulfill(route, {"materials": [MATERIAL]})
            elif path == "/api/materials/7":
                fulfill(route, {"material": MATERIAL})
            elif path == "/api/knowledge/search":
                searches.append(parse_qs(urlparse(request.url).query))
                fulfill(route, {"hits": [HIT], "index_state": "ready"})
            elif path == "/api/ask":
                body = request.post_data_json
                asks.append(body)
                question = body["question"]
                attempts[question] = attempts.get(question, 0) + 1
                if question.startswith("延迟回答") and attempts[question] == 1:
                    delayed.append(route)
                elif question == "先失败" and attempts[question] == 1:
                    fulfill(route, {"error": "问答服务暂时不可用"}, status=503)
                elif question == "无依据":
                    fulfill(route, {"answer": "资料中未找到相关内容", "citations": [], "index_state": "ready"})
                elif question == "安全展示":
                    fulfill(route, {"answer": "**安全回答** [1]\n<script>window.chatbotInjected = true</script>\n[危险链接](javascript:alert(1))",
                                    "citations": [HIT], "index_state": "ready"})
                else:
                    state = "building" if question == "索引中" else "degraded" if question == "索引异常" else "ready"
                    fulfill(route, {"answer": f"关于{question}，可以圈画关键词理解文章。[1]",
                                    "citations": [HIT], "index_state": state})
            else:
                raise AssertionError(f"Unexpected API path: {path}")

        def release_late():
            try:
                fulfill(delayed.pop(), {"answer": "迟到回答不应显示", "citations": [], "index_state": "ready"})
            except Error:
                # Browser may already have terminated the aborted network request.
                pass

        page.route("**/api/**", handle)
        page.goto(BASE_URL)
        page.wait_for_load_state("networkidle")
        # Inspect the rendered login controls before selecting them.
        assert "登录" in page.get_by_role("button").all_text_contents()
        page.get_by_label("账号", exact=True).fill("student_a1")
        page.get_by_label("密码", exact=True).fill("test-only-password")
        page.get_by_role("button", name="登录", exact=True).click()
        nav = page.get_by_role("navigation", name="主导航")
        expect(nav).to_be_visible()
        nav.get_by_role("button", name="知识问答").click()
        panel = page.get_by_role("region", name="本班知识问答")
        expect(panel).to_be_visible()
        assert page.get_by_role("button", name="发送问题").count() == 1
        expect(nav.get_by_role("button", name="知识问答")).to_have_attribute("aria-current", "page")
        composer = page.get_by_label("向本班资料提问", exact=True)
        send_button = page.get_by_role("button", name="发送问题", exact=True)
        expect(send_button).to_be_disabled()

        composer.fill("😀" * 200)
        expect(page.locator("#chat-input-hint span")).to_have_text("200/200")
        expect(send_button).to_be_enabled()
        composer.fill("😀" * 201)
        expect(send_button).to_be_disabled()
        composer.fill("中文输入")
        composer.dispatch_event("keydown", {"key": "Enter", "code": "Enter", "isComposing": True, "keyCode": 229, "bubbles": True})
        assert len(asks) == 0
        composer.press("Shift+Enter")
        assert "\n" in composer.input_value()
        composer.fill("")
        page.screenshot(path=str(OUTPUT / "chatbot-desktop-empty.png"), full_page=True)

        def send(question):
            composer.fill(question)
            composer.press("Enter")
            expect(page.get_by_role("button", name="发送问题", exact=True)).to_be_visible()

        for number in range(1, 5):
            send(f"问题{number}")
            expect(page.locator(".chat-round")).to_have_count(number)
        assert asks[0]["history"] == []
        assert len(asks[1]["history"]) == 2
        assert len(asks[3]["history"]) == 6
        assert asks[3]["history"][0]["content"] == "问题1"
        assert all(set(message) == {"role", "content"} for message in asks[3]["history"])
        print("PASS: Unicode/IME, consecutive messages and bounded history", flush=True)
        source = page.locator(".chat-round").last.get_by_role("button", name="[1] 文本细读教研笔记 · 切片 1", exact=True)
        source.click()
        expect(page.get_by_role("dialog")).to_be_visible()
        expect(page.locator(".document-body mark")).to_have_text(HIT["excerpt"])
        expect(page.get_by_role("button", name="下载原文件")).to_be_visible()
        page.get_by_role("button", name="关闭详情").click()
        page.screenshot(path=str(OUTPUT / "chatbot-desktop.png"), full_page=True)

        nav.get_by_role("button", name="材料库").click()
        expect(panel).to_be_hidden()
        page.get_by_label("搜索本班材料").fill("文本")
        expect(page.get_by_role("button", name="查看 文本细读教研笔记")).to_be_visible()
        page.wait_for_timeout(350)
        assert "文本" in material_queries
        nav.get_by_role("button", name="知识检索").click()
        page.get_by_label("检索本班知识片段").fill("文本细读")
        expect(page.get_by_role("button", name="打开来源")).to_be_visible()
        for mode, label in [("keyword", "关键字"), ("vector", "语义"), ("hybrid", "混合")]:
            page.get_by_role("button", name=label, exact=True).click()
            page.wait_for_timeout(350)
            assert searches[-1]["mode"] == [mode]
        nav.get_by_role("button", name="快捷命令").click()
        page.get_by_role("dialog", name="快捷命令").get_by_role("button", name="知识问答 AI").click()
        expect(page.locator(".chat-round")).to_have_count(4)
        print("PASS: sources, material/search modes, command navigation and retained conversation", flush=True)

        send("无依据")
        expect(page.locator(".chat-round").last).to_contain_text("资料中未找到相关内容")
        expect(page.locator(".chat-round").last.locator(".chat-sources")).to_have_count(0)
        send("索引中")
        expect(page.locator(".chat-round").last).to_contain_text("本班材料正在建立索引")
        send("索引异常")
        expect(page.locator(".chat-round").last).to_contain_text("部分材料索引失败")
        send("安全展示")
        expect(page.locator(".chat-round").last.locator("strong")).to_have_text("安全回答")
        assert page.evaluate("window.chatbotInjected") is None
        assert page.locator('.chat-answer a[href^="javascript:"]').count() == 0

        send("先失败")
        print("PASS: no evidence, index warnings and safe Markdown", flush=True)
        expect(page.get_by_role("alert")).to_contain_text("问答服务暂时不可用")
        before_retry = page.locator(".chat-round").count()
        page.get_by_role("button", name="重试此问题").click()
        expect(page.locator(".chat-round").last).to_contain_text("关于先失败")
        assert page.locator(".chat-round").count() == before_retry
        assert all(turn["content"] != "先失败" for turn in asks[-1]["history"])

        composer.fill("延迟回答1")
        composer.press("Enter")
        expect(page.get_by_role("button", name="停止回答")).to_be_visible()
        pending_calls = len(asks)
        composer.fill("不会重复发送")
        composer.press("Enter")
        assert len(asks) == pending_calls
        page.get_by_role("button", name="停止回答").click()
        expect(page.locator(".chat-round").last).to_contain_text("已停止回答")
        release_late()
        expect(panel).not_to_contain_text("迟到回答不应显示")
        page.get_by_role("button", name="重试此问题").click()
        expect(page.locator(".chat-round").last).to_contain_text("关于延迟回答1")

        composer.fill("延迟回答2")
        composer.press("Enter")
        expect(page.get_by_role("button", name="停止回答")).to_be_visible()
        page.get_by_role("button", name="清空对话").click()
        release_late()
        expect(page.locator(".chat-round")).to_have_count(0)
        expect(panel).not_to_contain_text("迟到回答不应显示")
        send("清空后的问题")
        assert asks[-1]["history"] == []

        composer.fill("延迟回答3")
        composer.press("Enter")
        expect(page.get_by_role("button", name="停止回答")).to_be_visible()
        nav.get_by_role("button", name="材料库").click()
        release_late()
        nav.get_by_role("button", name="知识问答").click()
        expect(page.locator(".chat-round").last).to_contain_text("已停止回答")
        expect(panel).not_to_contain_text("迟到回答不应显示")
        expect(page.locator(".chat-round")).to_have_count(2)
        print("PASS: failures/retry, stop/clear, late responses and inactive-view cancellation", flush=True)
        page.get_by_role("button", name="切换主题", exact=True).click()
        page.screenshot(path=str(OUTPUT / "chatbot-dark.png"), full_page=True)
        page.get_by_role("button", name="切换主题", exact=True).click()
        page.set_viewport_size({"width": 390, "height": 844})
        page.evaluate("window.scrollTo(0, 0)")
        expect(page.locator(".mobile-sections").get_by_role("button", name="知识问答")).to_be_visible()
        expect(send_button).to_be_visible()
        assert page.evaluate("document.documentElement.scrollWidth <= window.innerWidth")
        assert send_button.bounding_box()["y"] + send_button.bounding_box()["height"] <= 844
        page.screenshot(path=str(OUTPUT / "chatbot-mobile.png"), full_page=True)
        page.set_viewport_size({"width": 320, "height": 667})
        page.evaluate("window.scrollTo(0, 0)")
        assert page.evaluate("document.documentElement.scrollWidth <= window.innerWidth")
        assert send_button.bounding_box()["y"] + send_button.bounding_box()["height"] <= 667
        page.screenshot(path=str(OUTPUT / "chatbot-small-mobile.png"), full_page=True)

        page.reload()
        page.wait_for_load_state("networkidle")
        page.locator(".mobile-sections").get_by_role("button", name="知识问答").click()
        expect(page.locator(".chat-round")).to_have_count(0)
        composer.fill("延迟回答4")
        composer.press("Enter")
        expect(page.get_by_role("button", name="停止回答")).to_be_visible()
        page.get_by_role("button", name="退出登录").click()
        expect(page.get_by_role("button", name="登录", exact=True)).to_be_visible()
        release_late()
        assert page.evaluate("sessionStorage.getItem('campus-access-token-v2')") is None
        page.get_by_label("账号", exact=True).fill("student_a1")
        page.get_by_label("密码", exact=True).fill("test-only-password")
        page.get_by_role("button", name="登录", exact=True).click()
        page.locator(".mobile-sections").get_by_role("button", name="知识问答").click()
        expect(page.locator(".chat-round")).to_have_count(0)
        expect(panel).not_to_contain_text("迟到回答不应显示")
        assert not page_errors, page_errors
        browser.close()
        print(f"PASS: {len(asks)} mock ask requests; history, sources, limits/IME, retry/stop/clear, late responses, navigation, safe content, themes, mobile and logout.")


if __name__ == "__main__":
    run()
