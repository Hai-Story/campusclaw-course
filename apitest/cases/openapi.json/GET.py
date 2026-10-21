from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    data=c.expect('GET','/openapi.json',token='')
    assert data['openapi']=='3.0.3' and data['components']['securitySchemes']['bearerAuth']['scheme']=='bearer'
    assert '/api/conversations/{id}/messages' in data['paths']
    assert 'text/event-stream' in data['paths']['/api/conversations/{id}/messages']['post']['responses']['200']['content']

if __name__ == "__main__": standalone(run)
