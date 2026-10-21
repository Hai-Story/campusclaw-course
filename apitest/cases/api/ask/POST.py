from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    before=c.stats()['chat'];empty=c.expect('POST','/api/ask',{'question':'明日天气'})
    assert empty['answer']=='资料中未找到相关内容' and empty['citations']==[] and c.stats()['chat']==before
    assert empty['index_state'] in ('ready','building','degraded')
    answer=c.expect('POST','/api/ask',{'question':'方程去括号'})
    assert '[1]' in answer['answer'] and answer['citations']
    assert answer['index_state'] in ('ready','building','degraded')
    assert all(c.scalar('SELECT class_id FROM materials WHERE id=%s',(h['material_id'],))==c.a['class_id'] for h in answer['citations'])

if __name__ == "__main__": standalone(run)
