from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    assert c.scalar('SELECT class_id FROM materials WHERE id=%s',(c.material,))==c.a['class_id']
    assert c.scalar('SELECT COUNT(*) FROM knowledge_entries WHERE material_id=%s',(c.material,))==1
    before=c.scalar('SELECT COUNT(*) FROM materials')
    c.expect('POST','/api/materials',status=403,token=c.student,raw=b'bad',headers={'Content-Type':'multipart/form-data; boundary=x'})
    assert c.scalar('SELECT COUNT(*) FROM materials')==before

if __name__ == "__main__": standalone(run)
