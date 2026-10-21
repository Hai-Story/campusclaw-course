from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    path=f'/api/agents/{c.agent["id"]}'
    c.expect('PUT',path,c.agent_input,403,c.student)
    prior=c.rows('SELECT * FROM tutor_agents WHERE id=%s',(c.agent['id'],))[0]
    c.expect('PUT',path,dict(c.agent_input,prompt='should rollback',skill_ids=[c.b_skill['id']]),404)
    assert c.rows('SELECT * FROM tutor_agents WHERE id=%s',(c.agent['id'],))[0]==prior
    assert c.scalar('SELECT skill_id FROM tutor_agent_skills WHERE agent_id=%s',(c.agent['id'],))==c.skill['id']
    data=c.expect('PUT',path,dict(c.agent_input,prompt='新的角色提示词',material_ids=[c.material,c.unbound]))
    assert data['prompt']==c.scalar('SELECT prompt FROM tutor_agents WHERE id=%s',(c.agent['id'],))
    assert c.scalar('SELECT COUNT(*) FROM tutor_agent_materials WHERE agent_id=%s',(c.agent['id'],))==2
    c.expect('PUT',path,c.agent_input)
    c.same404('PUT',f'/api/agents/{c.b_agent["id"]}','/api/agents/99999999',payload=c.agent_input)


    # Delete a binding target in another transaction while the API attempts to bind it.
    import concurrent.futures,time,pymysql,os
    temporary=c.expect('POST','/api/skills',{'source':c.source.replace('名称：','名称：竞争')},201)
    connection=pymysql.connect(host=os.environ['APITEST_DB_HOST'],port=int(os.environ['APITEST_DB_PORT']),database=os.environ['APITEST_DB_NAME'],user=os.environ['APITEST_DB_USER'],password=c.password,autocommit=False)
    try:
        with connection.cursor() as cursor:cursor.execute('DELETE FROM tutor_skills WHERE id=%s',(temporary['id'],))
        with concurrent.futures.ThreadPoolExecutor() as pool:
            future=pool.submit(c.call,'PUT',path,dict(c.agent_input,prompt='must not partially commit',skill_ids=[c.skill['id'],temporary['id']]),c.teacher)
            time.sleep(.15);connection.commit()
            response=future.result(timeout=10);assert response[0]==404
        assert c.scalar('SELECT prompt FROM tutor_agents WHERE id=%s',(c.agent['id'],))==c.agent_input['prompt']
        assert c.scalar('SELECT skill_id FROM tutor_agent_skills WHERE agent_id=%s',(c.agent['id'],))==c.skill['id']
    finally:connection.rollback();connection.close()

if __name__ == "__main__": standalone(run)
