import os,json,pathlib,datetime
w=pathlib.Path(os.environ['CLI_WORK_DIR']);now=datetime.datetime.now(datetime.timezone.utc).isoformat().replace('+00:00','Z')
items={
'contacts':[{'id':1,'email':'reader@example.com','first_name':'Reader','unsubscribed_at':None},{'id':2,'email':'suppressed@example.com','unsubscribed_at':now}],
'unsubscribed':[{'id':2,'email':'suppressed@example.com','unsubscribed_at':now}],
'lists':[{'id':12,'name':'Newsletter'}], 'memberships':[{'contact_id':1,'list_id':12},{'contact_id':2,'list_id':12}],
 'tags':[{'id':3,'name':'Reader'}], 'contact_tags':[{'contact_id':1,'tag_id':3}], 'contact_fields':[],
 'campaigns':[{'id':42,'title':'Issue 42','subject':'A useful idea','status':'sent','lists':[12]}],
 'campaign_stats':[{'campaign_id':42,'sent_count':2,'unique_open_count':1,'unique_click_count':1,'open_rate':50,'click_rate':50,'link_stats':[{'url':'https://example.com/story','clicks':1}]}],
 'engagement':[{'campaign_id':42,'type':'non_openers','contact_id':1,'email':'reader@example.com'},{'campaign_id':42,'type':'non_openers','contact_id':2,'email':'suppressed@example.com'}],
 'activity':[{'contact_id':1,'campaign_id':42,'type':'sent'}], 'forms':[],
 'domains':[{'id':8,'domain':'example.com','validated_at':now,'dns':{}}], 'automations':[], 'automation_emails':[]}
s={'schema_version':1,'resources':{k:{'items':v,'source':'synthetic fixture; no account data','observed_at':now,'complete':True,'capped':False,'total':len(v),'pages':1,'scopes_complete':True} for k,v in items.items()},
 'campaign':{'id':42,'title':'Issue 43','subject':'A useful idea','html':'<p>Hello <a href="https://example.com/story">reader</a></p>','from_name':'Example Newsletter','from_email':'newsletter@example.com','lists':[12]},
 'automation':{'title':'Welcome','trigger_type':'apply_list','trigger_list_id':12,'active':False,'emails':[{'subject':'Welcome','html':'<p>Hello</p>','from_name':'Example Newsletter','from_email':'newsletter@example.com'},{'subject':'Next idea','html':'<p>Next</p>','from_name':'Example Newsletter','from_email':'newsletter@example.com'}]}}
kit={'schema_version':1,'resources':{k:dict(v) for k,v in s['resources'].items() if k in ['contacts','unsubscribed','lists','tags','contact_fields','forms','automations']}}
s['kit']=kit;s['previous']=kit;s['mappings']=[{'kind':'lists','source':'12','target':'12','approved':True},{'kind':'tags','source':'3','target':'3','approved':True}]
(w/'examples/snapshot.json').write_text(json.dumps(s,indent=2)+'\n');(w/'examples/kit-snapshot.json').write_text(json.dumps(kit,indent=2)+'\n');(w/'examples/contacts.csv').write_text('email,first_name,last_name,status\nreader@example.com,Reader,Example,active\nnew@example.com,New,Reader,active\nsuppressed@example.com,Suppressed,Example,unsubscribed\n')
