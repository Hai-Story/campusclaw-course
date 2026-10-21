from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    before=c.scalar('SELECT COUNT(*) FROM tutor_skills')
    c.expect('POST','/api/skills',{'source':c.source},403,c.student)
    c.expect('POST','/api/skills',{'source':c.source},400)
    for text in [c.source.replace('启用：是','启用：maybe'),c.source.replace('knowledge_search','shell'),'名称：重复\n'+c.source,c.source.replace('名称：','未知：'),c.source.split('\n\n')[0]]:c.expect('POST','/api/skills',{'source':text},400)
    assert c.scalar('SELECT COUNT(*) FROM tutor_skills')==before
    row=c.rows('SELECT * FROM tutor_skills WHERE id=%s',(c.skill['id'],))[0]
    assert row['source_text']==c.source and row['enabled']==1

if __name__ == "__main__": standalone(run)
