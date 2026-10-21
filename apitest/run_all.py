#!/usr/bin/env python3
"""Discover per-interface cases and fail closed if setup or any assertion fails."""
import html
import json
import os
import runpy
import sys
import time
from pathlib import Path
from support import Context


def main():
    root=Path(__file__).resolve().parent
    report=root/"reports";report.mkdir(exist_ok=True)
    results=[];c=None;failed=False
    try:
        c=Context()
        document=c.expect("GET","/openapi.json",token="")
        cases=sorted((root/"cases").rglob("*.py"))
        available={str(p.relative_to(root/"cases").parent).replace("\\","/")+" "+p.stem for p in cases}
        required={path.strip("/")+" "+method.upper() for path,methods in document["paths"].items() for method in methods if path.startswith("/api/")}
        missing=required-available
        assert not missing,"缺少独立接口脚本："+", ".join(sorted(missing))
        for path in cases:
            begin=time.monotonic();name=str(path.relative_to(root));offset=len(c.events)
            try: runpy.run_path(str(path),run_name="apitest_case")["run"](c);state="PASS";message="响应、内容及数据库断言通过"
            except Exception as error: failed=True;state="FAIL";message=f"{type(error).__name__}: {error}"
            for secret in [c.password,*c.tokens,os.environ.get("APITEST_MCP_TOKEN", "") ]:
                if secret: message=message.replace(secret,"[REDACTED]")
            results.append(dict(case=name,status=state,duration=round(time.monotonic()-begin,3),message=message,evidence=c.events[offset:]))
            print(state,name)
    except Exception as error:
        failed=True
        # Setup failures deliberately avoid printing exception payloads containing connection values.
        results.append(dict(case="环境预检",status="FAIL",duration=0,message=type(error).__name__+"：测试应用、独立数据库或必需用例未准备完成",evidence=[]))
        print("FAIL 环境预检；见 reports/index.html")
    finally:
        if c:c.close()
    rows=[]
    for result in results:
        rows.append('<tr><td>'+html.escape(result['case'])+'</td><td>'+result['status']+'</td><td>'+str(result['duration'])+'</td><td>'+html.escape(result['message'])+'<details><summary>脱敏请求/响应证据</summary><pre>'+html.escape(json.dumps(result['evidence'],ensure_ascii=False,indent=2))+'</pre></details></td></tr>')
    document='<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>CampusClaw 接口验收</title><style>body{font-family:system-ui;padding:32px;color:#222}table{border-collapse:collapse;width:100%}td,th{border:1px solid #ddd;padding:12px;text-align:left}pre{white-space:pre-wrap}h1{color:#8f1d22}</style><h1>CampusClaw 接口验收 · '+('未通过' if failed else '通过')+'</h1><p>使用隔离测试数据库与模拟网关；不代表真实模型或同伴验收。</p><table><tr><th>接口脚本</th><th>结果</th><th>秒</th><th>断言与脱敏证据</th></tr>'+''.join(rows)+'</table></html>'
    (report/"index.html").write_text(document)
    print(f"{sum(r['status']=='PASS' for r in results)}/{len(results)} 通过；报告：apitest/reports/index.html")
    return 1 if failed else 0

if __name__=="__main__":sys.exit(main())
