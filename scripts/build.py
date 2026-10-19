#!/usr/bin/env python3
"""Build a reproducible Linux/arm64 Lambda archive from any host OS."""
from pathlib import Path
import os, subprocess, zipfile
ROOT=Path(__file__).resolve().parents[1]
def main():
 out=ROOT/'build'; out.mkdir(exist_ok=True)
 env=os.environ.copy(); env.update(GOOS='linux', GOARCH='arm64', CGO_ENABLED='0')
 subprocess.run(['go','build','-trimpath','-ldflags=-s -w','-o',str(out/'bootstrap'),'./cmd/lambda'],cwd=ROOT/'backend',env=env,check=True)
 info=zipfile.ZipInfo('bootstrap'); info.create_system=3; info.external_attr=0o100755 << 16; info.compress_type=zipfile.ZIP_DEFLATED
 with zipfile.ZipFile(out/'lambda.zip','w') as archive: archive.writestr(info,(out/'bootstrap').read_bytes())
 print('Lambda archive ready: build/lambda.zip')
if __name__=='__main__': main()
