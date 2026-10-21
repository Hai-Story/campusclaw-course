from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    code,data,_=c.call('GET',f'/api/materials/{c.material}/file',token=c.student)
    assert code==200 and data==c.scalar('SELECT content FROM knowledge_entries WHERE material_id=%s',(c.material,))
    c.same404('GET',f'/api/materials/{c.b_material}/file','/api/materials/99999999/file',c.student)

if __name__ == "__main__": standalone(run)
