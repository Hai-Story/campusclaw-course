"""Verify preset apps and exact local bindings through API and MySQL."""
from support import Context

def main():
    c=Context()
    try:
        expected={'数学解题助手':'数学解题引导','语文阅读助手':'语文阅读分析','英语学习助手':'英语学习指导'}
        for token,class_id in ((c.student,c.a['class_id']),(c.other,c.b['class_id'])):
            agents=c.expect('GET','/api/agents',token=token)['agents']
            for name,skill_name in expected.items():
                matches=[a for a in agents if a['name']==name]
                assert len(matches)==1
                agent=matches[0]
                assert agent['class_id']==class_id and agent['enabled'] and agent['material_ids']==[] and len(agent['skill_ids'])==1
                assert c.scalar('SELECT name FROM tutor_skills WHERE id=%s AND class_id=%s',(agent['skill_ids'][0],class_id))==skill_name
                conv=c.expect('POST','/api/conversations',{'agent_id':agent['id']},201,token)
                events=c.stream('请提供一些学习建议',conversation=conv['id'],token=token)
                c.assert_saved(events)
                assert events[0][1]['citations']==[]
        print('PASS three built-in apps in both classes: local bindings and no-material conversations')
    finally:c.close()

if __name__=='__main__':main()
