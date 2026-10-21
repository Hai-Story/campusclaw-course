from pathlib import Path
import sys
sys.path.insert(0, str(next(p for p in Path(__file__).resolve().parents if p.name == "apitest")))
from support import standalone

def run(c):

    code,data,_=c.call('GET','/health');assert code==200 and data=='ok\n'

if __name__ == "__main__": standalone(run)
