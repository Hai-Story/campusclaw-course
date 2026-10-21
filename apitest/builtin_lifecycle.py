"""Restart lifecycle checks, exclusively in an authorized isolated database."""
import json
import os
import sys
import uuid
from pathlib import Path
import pymysql

assert os.environ.get('APITEST_ALLOW_WRITES')=='1' and os.environ['APITEST_DB_NAME'].startswith('campusclaw_test_')
db=pymysql.connect(host=os.environ['APITEST_DB_HOST'],port=int(os.environ['APITEST_DB_PORT']),user=os.environ['APITEST_DB_USER'],password=os.environ['APITEST_PASSWORD'],database=os.environ['APITEST_DB_NAME'],autocommit=True,cursorclass=pymysql.cursors.DictCursor)
state_path=Path(__file__).parent/'reports/builtin-lifecycle.json'
mode=sys.argv[1]
with db.cursor() as cur:
    if mode=='prepare':
        cur.execute('INSERT INTO classes(name) VALUES(%s)',('默认内容验收-'+uuid.uuid4().hex[:12],))
        state={'class_id':cur.lastrowid}
        state_path.write_text(json.dumps(state))
        print('Prepared isolated lifecycle class; restart test API next')
    else:
        state=json.loads(state_path.read_text());class_id=state['class_id']
        if mode=='mutate':
            for table in ['tutor_agents','tutor_skills','tutor_prompt_templates']:
                cur.execute('SELECT COUNT(*) AS n FROM '+table+' WHERE class_id=%s',(class_id,));assert cur.fetchone()['n']==3
            cur.execute("UPDATE tutor_agents SET name='教师改名后的数学助手',enabled=0,prompt='教师自己的提示词' WHERE class_id=%s AND name='数学解题助手'",(class_id,))
            cur.execute("UPDATE tutor_skills SET enabled=0 WHERE class_id=%s AND name='语文阅读分析'",(class_id,))
            cur.execute("DELETE FROM tutor_skills WHERE class_id=%s AND name='英语学习指导'",(class_id,))
            print('Changed defaults only in isolated lifecycle class; restart test API next')
        elif mode=='check':
            cur.execute('SELECT name,prompt,enabled FROM tutor_agents WHERE class_id=%s ORDER BY id',(class_id,));agents=cur.fetchall()
            assert len(agents)==3 and agents[0]==dict(name='教师改名后的数学助手',prompt='教师自己的提示词',enabled=0)
            cur.execute('SELECT name,enabled FROM tutor_skills WHERE class_id=%s ORDER BY id',(class_id,));skills=cur.fetchall()
            assert len(skills)==2 and skills[1]['name']=='语文阅读分析' and skills[1]['enabled']==0
            cur.execute('SELECT COUNT(*) AS n FROM tutor_prompt_templates WHERE class_id=%s',(class_id,));assert cur.fetchone()['n']==3
            print('PASS restart preserves rename, prompt edit, disable and skill deletion; no duplicates')
        else:raise ValueError('unknown lifecycle mode')
db.close()
