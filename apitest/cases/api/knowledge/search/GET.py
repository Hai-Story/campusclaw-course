from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    for mode in ['keyword','vector','hybrid']:
        before=c.stats()['embeddings']
        data=c.expect('GET',f'/api/knowledge/search?q=方程&mode={mode}&class_id={c.b["class_id"]}',token=c.student)
        assert data['hits'] and all(c.scalar('SELECT class_id FROM knowledge_chunks WHERE id=%s',(h['chunk_id'],))==c.a['class_id'] for h in data['hits'])
        if mode=='keyword':assert c.stats()['embeddings']==before
        for hit in data['hits']:assert c.scalar('SELECT chunk_text FROM knowledge_chunks WHERE id=%s',(hit['chunk_id'],))==hit['excerpt']
    empty=c.expect('GET','/api/knowledge/search?q=明日天气&mode=hybrid')
    assert empty['hits']==[] and empty['message']=='资料中未找到相关内容'
    assert c.call('GET','/api/knowledge/search?q=方程')[0]==401

if __name__ == "__main__": standalone(run)
