"""Compare public application data around an API restart/migration, never credentials."""
import hashlib
import json
import os
import sys
from pathlib import Path
import pymysql

if not os.environ["APITEST_DB_NAME"].startswith("campusclaw_test_"):raise RuntimeError("requires isolated test database")
db=pymysql.connect(host=os.environ["APITEST_DB_HOST"],port=int(os.environ["APITEST_DB_PORT"]),user=os.environ["APITEST_DB_USER"],password=os.environ["APITEST_PASSWORD"],database=os.environ["APITEST_DB_NAME"])
queries={"users":"SELECT id,username,role,class_id FROM users ORDER BY id", "materials":"SELECT id,class_id,title,stored_name FROM materials ORDER BY id", "knowledge":"SELECT id,material_id,class_id,content FROM knowledge_entries ORDER BY id", "agents":"SELECT id,class_id,name,prompt,enabled FROM tutor_agents ORDER BY id", "skills":"SELECT id,class_id,source_text,enabled FROM tutor_skills ORDER BY id", "conversations":"SELECT id,user_id,class_id,agent_id,title FROM tutor_conversations ORDER BY id", "messages":"SELECT id,conversation_id,role,content,citations_json,skill_name,status FROM tutor_messages ORDER BY id", "skill_bindings":"SELECT agent_id,skill_id FROM tutor_agent_skills ORDER BY agent_id,skill_id", "material_bindings":"SELECT agent_id,material_id FROM tutor_agent_materials ORDER BY agent_id,material_id"}
if os.environ.get("APITEST_MCP_TOKEN"):
    queries.update({
        "mcp_servers":"SELECT id,class_id,name,base_url,status,revision FROM mcp_servers ORDER BY id",
        "mcp_tools":"SELECT server_id,name,descriptor_hash FROM mcp_tools ORDER BY server_id,name",
        "mcp_approvals":"SELECT skill_id,server_id,tool_name,descriptor_hash FROM mcp_tool_approvals ORDER BY skill_id,server_id,tool_name",
        "mcp_usage":"SELECT conversation_id,attempts FROM mcp_conversation_usage ORDER BY conversation_id",
        "mcp_audits":"SELECT id,server_id,call_id,status FROM mcp_call_audits ORDER BY id",
        "mcp_changes":"SELECT id,server_id,tool_name,old_json,new_json FROM mcp_tool_changes ORDER BY id",
        "message_tools":"SELECT id,tool_calls_json FROM tutor_messages ORDER BY id",
    })
snapshot={}
with db.cursor() as cur:
    for name,query in queries.items():
        cur.execute(query);rows=cur.fetchall();snapshot[name]={"count":len(rows),"hash":hashlib.sha256(json.dumps(rows,ensure_ascii=False,default=str).encode()).hexdigest()}
db.close()
path=Path(__file__).resolve().parent/"reports/migration-snapshot.json"
if "--before" in sys.argv:path.write_text(json.dumps(snapshot));print("Saved migration data fingerprints")
else:
    assert json.loads(path.read_text())==snapshot,"Migration/restart changed application data"
    print("PASS repeated migration: users, originals, agents, skills, bindings and history preserved")
