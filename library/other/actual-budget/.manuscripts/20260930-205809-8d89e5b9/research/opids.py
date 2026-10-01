import json,sys,re
p=sys.argv[1]; s=json.load(open(p))
special={('post','merge'):'merge',('put','close'):'close',('put','reopen'):'reopen',('get','balance'):'balance',
 ('get','balancehistory'):'balance-history',('post','banksync'):'bank-sync',('post','batch'):'create-batch',('delete','batch'):'delete-batch',
 ('post','import'):'import',('post','nextmonthbudgethold'):'hold',('delete','nextmonthbudgethold'):'reset-hold',('post','categorytransfers'):'transfer',
 ('get','export'):'export',('get','id-by-name'):'id-by-name',('get','payees-common'):'common',('post','run-query'):'run-query',
 ('get','preferences'):'preferences',('get','actualserverversion'):'server-version',('get','actualhttpapiversion'):'sidecar-version',('get','rules'):'rules'}
out={}
for path,ops in s['paths'].items():
    segs=[x for x in path.strip('/').split('/') if x]
    for m,op in ops.items():
        if m not in ('get','post','put','patch','delete'): continue
        last=segs[-1]; isparam=last.startswith('{')
        prev=segs[-2] if len(segs)>1 else ''
        if path.startswith('/budgets/{budgetSyncId}/notes/'):
            kind=segs[3]; oid={'get':'get','put':'set','delete':'delete'}[m]+'-'+{'budgetmonth':'month'}.get(kind,kind)
        elif (m,last) in special: oid=special[(m,last)]
        elif path=='/budgets/import': oid='import'
        elif path=='/budgets': oid='list'
        elif isparam: oid={'get':'get','patch':'update','delete':'delete','put':'set'}[m]
        else: oid={'get':'list','post':'create'}.get(m,m)
        op['operationId']=oid; out[(m.upper(),path)]=oid
json.dump(s,open(p,'w'),indent=2)
for (m,pth),o in sorted(out.items(), key=lambda x:x[0][1]): print(f"{m:6} {pth:75} {o}")
