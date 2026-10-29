#!/usr/bin/env python3
"""Install pinned upstream Fabric and CoreDNS tools for Linux amd64."""
import argparse,hashlib,json,platform,tarfile,urllib.request
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
VERSION='2.4.9'
FABRIC_SHA256='15ffdf9a5367befd37de732f6c71100c555c1aa4e622fc97c156bbeeff8d396a'
DNS_SHA256='c835bd5e9ba9a98df9631e5a3f0effc60f03ccb9c687c91ae2c85ebf7ab1fc34'
def download(url,path):
 if not path.exists():
  path.parent.mkdir(parents=True,exist_ok=True);temporary=path.with_suffix('.partial')
  with urllib.request.urlopen(url,timeout=60) as response,temporary.open('wb') as output:
   while chunk:=response.read(1<<20):output.write(chunk)
  temporary.rename(path)
 return path
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--archive',type=Path,help='Official Fabric release archive');parser.add_argument('--sha256',default=FABRIC_SHA256,help='Expected Fabric archive checksum');parser.add_argument('--coredns-archive',type=Path,help='Official CoreDNS 1.12.0 archive');args=parser.parse_args()
 if platform.system()!='Linux' or platform.machine() not in ('x86_64','amd64'):raise SystemExit('This setup targets Linux amd64')
 target=ROOT/'runtime/tools/fabric';target.mkdir(parents=True,exist_ok=True)
 url=f'https://github.com/hyperledger/fabric/releases/download/v{VERSION}/hyperledger-fabric-linux-amd64-{VERSION}.tar.gz'
 archive=args.archive or download(url,ROOT/f'runtime/downloads/fabric-{VERSION}.tar.gz')
 digest=hashlib.sha256(archive.read_bytes()).hexdigest()
 if args.sha256 and digest.lower()!=args.sha256.lower():raise SystemExit('Fabric archive checksum mismatch')
 with tarfile.open(archive) as tar:
  for member in tar.getmembers():
   parts=Path(member.name).parts
   if not parts or parts[0] not in ('bin','config'):continue
   path=(target/member.name).resolve()
   if not path.is_relative_to(target.resolve()) or member.issym() or member.islnk():raise SystemExit('Unsupported archive entry')
   if member.isdir():path.mkdir(parents=True,exist_ok=True)
   elif member.isfile():
    path.parent.mkdir(parents=True,exist_ok=True)
    with tar.extractfile(member) as source:path.write_bytes(source.read())
    path.chmod(member.mode & 0o755)
 (target/'provenance.json').write_text(json.dumps(dict(version=VERSION,url=url,archive_sha256=digest),indent=2))
 dnsurl='https://github.com/coredns/coredns/releases/download/v1.12.0/coredns_1.12.0_linux_amd64.tgz'
 dnsarchive=args.coredns_archive or download(dnsurl,ROOT/'runtime/downloads/coredns-1.12.0.tgz')
 dnsdigest=hashlib.sha256(dnsarchive.read_bytes()).hexdigest()
 if dnsdigest!=DNS_SHA256:raise SystemExit('CoreDNS checksum mismatch')
 with tarfile.open(dnsarchive) as tar:
  member=tar.getmember('coredns')
  if not member.isfile():raise SystemExit('Invalid CoreDNS archive')
  binary=ROOT/'runtime/tools/coredns'
  with tar.extractfile(member) as source:binary.write_bytes(source.read())
  binary.chmod(0o755)
 print('Installed Fabric',VERSION,'and CoreDNS 1.12.0 under runtime/tools')
 print('Fabric archive SHA256:',digest)
if __name__=='__main__':main()
