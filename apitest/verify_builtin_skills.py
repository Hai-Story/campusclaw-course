"""Checks defaults and preservation only in the authorized isolated test DB."""
import json
from support import Context

def main():
    c=Context()
    try:
        names=['数学解题引导','语文阅读分析','英语学习指导']
        for class_id in (c.a['class_id'],c.b['class_id']):
            row=c.rows("SELECT objects_json FROM builtin_initializations WHERE class_id=%s AND version='skills-v1'",(class_id,))[0]
            assert len(json.loads(row['objects_json']))==3
            for name in names:
                rows=c.rows('SELECT tools_json FROM tutor_skills WHERE class_id=%s AND name=%s',(class_id,name))
                assert len(rows)==1 and json.loads(rows[0]['tools_json'])==['knowledge_search']
        assert c.scalar("SELECT CHARACTER_MAXIMUM_LENGTH FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='tutor_skills' AND column_name='name'")==64
        assert c.scalar("SELECT CHARACTER_MAXIMUM_LENGTH FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='tutor_messages' AND column_name='skill_name'")==64
        print('PASS built-in skills: two classes, tools and 64-character migration')
    finally:c.close()

if __name__=='__main__':main()
