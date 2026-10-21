from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    c.expect('DELETE',f'/api/materials/{c.material}',status=403,token=c.student)
    c.same404('DELETE',f'/api/materials/{c.b_material}','/api/materials/99999999')
    temporary=c.upload('临时删除'+c.suffix,'方程建模测试材料')
    for _ in range(60):
        if c.scalar("SELECT COUNT(*) FROM knowledge_chunks WHERE material_id=%s AND index_status='ready'",(temporary,)):break
        import time;time.sleep(.1)
    c.expect('PUT',f'/api/agents/{c.agent["id"]}',dict(c.agent_input,material_ids=[c.material,temporary]))
    c.expect('DELETE',f'/api/materials/{temporary}',status=204)
    assert c.scalar('SELECT COUNT(*) FROM materials WHERE id=%s',(temporary,))==0
    assert c.scalar('SELECT COUNT(*) FROM tutor_agent_materials WHERE material_id=%s',(temporary,))==0
    assert c.scalar('SELECT material_id FROM tutor_agent_materials WHERE agent_id=%s',(c.agent['id'],))==c.material

if __name__ == "__main__": standalone(run)
