from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if p.name=="apitest")))
from support import standalone
from mcp_support import run_case

def run(c):
    run_case(c, "GET /api/mcp/servers/{id}")

if __name__=="__main__":standalone(run)
