from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    items=c.expect('GET','/api/materials',token=c.student)['materials']
    assert any(x['id']==c.material for x in items) and all(x['class_id']==c.a['class_id'] for x in items)
    assert all(x['id']!=c.b_material for x in items)

if __name__ == "__main__": standalone(run)
