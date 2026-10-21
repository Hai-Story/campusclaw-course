from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    import base64,hashlib,json
    code,data,headers=c.call('POST','/api/login',{'username':'teacher_a','password':c.password})
    assert code==200 and data['token_type']=='Bearer' and data['expires_in']>0
    assert not any(k.lower()=='set-cookie' for k in headers)
    c.tokens.append(data['token'])
    part=data['token'].split('.')[1];claims=json.loads(base64.urlsafe_b64decode(part+'='*((4-len(part)%4)%4)))
    assert c.scalar('SELECT user_id FROM sessions WHERE token_hash=%s',(hashlib.sha256(claims['jti'].encode()).hexdigest(),))==data['user']['id']
    unknown=c.call('POST','/api/login',{'username':'unknown_'+c.suffix,'password':'wrong'})
    wrong=c.call('POST','/api/login',{'username':'teacher_a','password':'wrong'})
    assert unknown[:2]==wrong[:2] and wrong[0]==401

if __name__ == "__main__": standalone(run)
