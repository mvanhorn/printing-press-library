// Minimal test-fixture seed for SF360 live-verification.
// Creates an Account, Contacts, Opportunities, a Case, and a Task in the
// target sandbox. Edit the constants below if your org has custom validation
// rules that reject the defaults — run scripts/seed-discover.sh first to
// identify required fields.
//
// Usage: sf apex run --file scripts/seed-minimal.apex --target-org <alias>
//
// After running, grep the debug log for ">>> ACME_ID=" and ">>> OPP_ID="
// and export those as environment variables before running live-verify.sh.

// ---------- Customize here if your org rejects the defaults ----------
String acctName       = 'Acme Corp SF360 Test';
String acctIndustry   = 'Technology/Software';  // must be a valid Industry picklist value in your org
String oppStageStart  = 'Prospecting';          // must be a valid starting StageName for new Opps
Integer contactCount  = 6;
// ---------- End customization ----------

Account a = new Account(
    Name = acctName,
    Description = 'Seeded for SF360 live verification',
    Industry = acctIndustry
);
insert a;

List<Contact> contacts = new List<Contact>();
for (Integer i = 1; i <= contactCount; i++) {
    contacts.add(new Contact(
        AccountId = a.Id,
        FirstName = 'Test' + i,
        LastName = 'Contact' + i,
        Email = 'test' + i + '@example.com'
    ));
}
insert contacts;

Id me = UserInfo.getUserId();
List<Opportunity> opps = new List<Opportunity>{
    new Opportunity(
        AccountId = a.Id,
        Name = 'SF360 Test Expansion',
        StageName = oppStageStart,
        CloseDate = Date.today().addDays(30)
    ),
    new Opportunity(
        AccountId = a.Id,
        Name = 'SF360 Test Renewal',
        StageName = oppStageStart,
        CloseDate = Date.today().addDays(60)
    )
};
insert opps;

insert new Case(
    AccountId = a.Id,
    ContactId = contacts[0].Id,
    Subject = 'SF360 seeded support ticket',
    Status = 'New',
    Origin = 'Email'
);

insert new Task(
    WhatId = a.Id,
    WhoId = contacts[0].Id,
    Subject = 'SF360 seeded discovery call',
    Status = 'Completed',
    ActivityDate = Date.today().addDays(-5)
);

System.debug('>>> ACME_ID=' + a.Id);
System.debug('>>> OPP_ID=' + opps[0].Id);
System.debug('>>> SEED_COMPLETE name=' + acctName + ' contacts=' + contacts.size() + ' opps=' + opps.size());
