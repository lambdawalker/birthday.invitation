#!/usr/bin/env python3
"""Interactive AWS SSO + private S3 Pulumi state deployment. No Pulumi account."""
from pathlib import Path
import getpass, json, os, re, secrets, shutil, subprocess, sys
ROOT=Path(__file__).resolve().parents[1]; INFRA=ROOT/'infra'
def run(args,cwd=INFRA,capture=False,input=None,check=True):
 # Windows requires the .cmd shim for npm; resolve explicitly, never shell=True.
 args=[shutil.which(args[0]) or args[0],*args[1:]]
 return subprocess.run(args,cwd=cwd,input=input,text=True,check=check,stdout=subprocess.PIPE if capture else None,stderr=subprocess.PIPE if capture else None)
def ask(prompt,default=''):
 answer=input(f'{prompt}'+(f' [{default}]' if default else '')+': ').strip();return answer or default
def step(n,title):print(f'\n\033[1;32m{n}  {title}\033[0m\n'+ '─'*55)
def valid_bucket(name):return bool(re.fullmatch(r'[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]',name)) and '..' not in name
def main():
 print('\n✦  ABI · Despliegue de la aldea\n   AWS + Cloudflare · estado privado en S3\n')
 step(1,'Revisar herramientas')
 missing=[c for c in ['aws','pulumi','go','node','npm'] if not shutil.which(c)]
 if missing:raise RuntimeError('Instala primero: '+', '.join(missing)+'. Consulta README.md → Despliegue.')
 step(2,'Conectar con AWS')
 profile=ask('Perfil AWS SSO','default');os.environ['AWS_PROFILE']=profile
 if run(['aws','sts','get-caller-identity'],capture=True,check=False).returncode:
  print('Si aún no configuraste el perfil, ejecuta aws configure sso primero.')
  run(['aws','sso','login','--profile',profile])
 identity=json.loads(run(['aws','sts','get-caller-identity'],capture=True).stdout)
 print('Cuenta AWS:',identity['Account'])
 region=ask('Región AWS','us-east-1');os.environ['AWS_REGION']=region;os.environ['AWS_DEFAULT_REGION']=region
 step(3,'Preparar estado Pulumi privado')
 bucket=ask('Bucket de estado',f'abi-pulumi-{identity["Account"]}-{region}')
 if not valid_bucket(bucket):raise RuntimeError('Nombre de bucket inválido')
 head=run(['aws','s3api','head-bucket','--bucket',bucket],capture=True,check=False)
 if head.returncode:
  if '404' not in head.stderr and 'Not Found' not in head.stderr:raise RuntimeError('No se pudo acceder al bucket. Revisa permisos; no se intentará crearlo a ciegas.')
  if ask('Crear el bucket de estado indicado (genera recursos AWS)? Escribe CREAR')!='CREAR':return
  args=['aws','s3api','create-bucket','--bucket',bucket]
  if region!='us-east-1':args+=['--create-bucket-configuration',f'LocationConstraint={region}']
  run(args,capture=True)
 run(['aws','s3api','put-public-access-block','--bucket',bucket,'--public-access-block-configuration','BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true'],capture=True)
 run(['aws','s3api','put-bucket-versioning','--bucket',bucket,'--versioning-configuration','Status=Enabled'],capture=True)
 run(['aws','s3api','put-bucket-encryption','--bucket',bucket,'--server-side-encryption-configuration',json.dumps({'Rules':[{'ApplyServerSideEncryptionByDefault':{'SSEAlgorithm':'AES256'}}]})],capture=True)
 os.environ['PULUMI_CONFIG_PASSPHRASE']=getpass.getpass('Frase de cifrado Pulumi (guárdala; usa la misma al actualizar): ')
 if not os.environ['PULUMI_CONFIG_PASSPHRASE']:raise RuntimeError('La frase no puede estar vacía')
 run(['pulumi','login',f's3://{bucket}/birthday-invitation'])
 stack=ask('Stack','production')
 if not re.fullmatch(r'[a-zA-Z0-9_-]+',stack):raise RuntimeError('Nombre de stack inválido')
 selected=run(['pulumi','stack','select',stack],capture=True,check=False)
 if selected.returncode:run(['pulumi','stack','init',stack,'--secrets-provider','passphrase'])
 step(4,'Dominio y secretos')
 domain=ask('Dominio','xitlaliabigail.com');zone=ask('Cloudflare Zone ID (Overview → API)')
 os.environ['CLOUDFLARE_API_TOKEN']=getpass.getpass('Token Cloudflare (Zone DNS Edit y Zone Read, solo esta zona): ')
 if not zone or not os.environ['CLOUDFLARE_API_TOKEN']:raise RuntimeError('Faltan datos de Cloudflare')
 for k,v in [('aws:region',region),('domain',domain),('cloudflareZoneId',zone)]:run(['pulumi','config','set',k,v])
 for name in ['adminKey','importNamespace']:
  if run(['pulumi','config','get',name],capture=True,check=False).returncode:
   run(['pulumi','config','set',name,'--secret'],input=secrets.token_urlsafe(40)+'\n',capture=True)
 print('Claves generadas y cifradas. No se mostrarán en registros.')
 step(5,'Compilar y revisar')
 run([sys.executable,str(ROOT/'scripts/build.py')],cwd=ROOT)
 run(['npm','ci','--no-audit','--no-fund'])
 run(['npm','run','check'])
 run(['pulumi','preview','--diff'])
 step(6,'Desplegar')
 if ask('Revisa los cambios arriba. Escribe DESPLEGAR para aplicarlos')!='DESPLEGAR':print('Sin cambios en la aplicación. El bucket de estado puede haber sido preparado.');return
 run(['pulumi','up','--yes','--skip-preview'])
 print('\n✓ Listo. Abre https://'+domain+'/admin.html')
 print('Para consultar tu clave de administrador en privado: cd infra && pulumi config get adminKey')
 print('Usa pulumi config get importNamespace para IMPORT_NAMESPACE y pulumi stack output tableName para TABLE_NAME.')
 print('El certificado y CloudFront pueden tardar varios minutos en estar disponibles.')
if __name__=='__main__':
 try:main()
 except (KeyboardInterrupt,EOFError):print('\nCancelado.');sys.exit(130)
 except (RuntimeError,subprocess.CalledProcessError) as e:print('\nNo se completó el despliegue:',e,file=sys.stderr);sys.exit(1)
