// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

// Package actualtest builds small but realistic Actual budget databases for
// tests. The schema mirrors the columns the CLI reads from a real
// db.sqlite (actualbudget/actual loot-core migrations), including the quirks:
// transactions.acct / description (payee id), integer dates, tombstones,
// payee_mapping / category_mapping, split parents and children, transfers.
package actualtest

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE accounts (id TEXT PRIMARY KEY, name TEXT, offbudget INTEGER DEFAULT 0, closed INTEGER DEFAULT 0, sort_order REAL, tombstone INTEGER DEFAULT 0, account_group_id TEXT);
CREATE TABLE category_groups (id TEXT PRIMARY KEY, name TEXT, is_income INTEGER DEFAULT 0, sort_order REAL, tombstone INTEGER DEFAULT 0, hidden INTEGER DEFAULT 0);
CREATE TABLE categories (id TEXT PRIMARY KEY, name TEXT, is_income INTEGER DEFAULT 0, cat_group TEXT, sort_order REAL, tombstone INTEGER DEFAULT 0, hidden INTEGER DEFAULT 0, goal_def TEXT);
CREATE TABLE category_mapping (id TEXT PRIMARY KEY, transferId TEXT);
CREATE TABLE payees (id TEXT PRIMARY KEY, name TEXT, category TEXT, tombstone INTEGER DEFAULT 0, transfer_acct TEXT, favorite INTEGER DEFAULT 0, learn_categories INTEGER DEFAULT 1);
CREATE TABLE payee_mapping (id TEXT PRIMARY KEY, targetId TEXT);
CREATE TABLE transactions (id TEXT PRIMARY KEY, isParent INTEGER DEFAULT 0, isChild INTEGER DEFAULT 0, acct TEXT, category TEXT, amount INTEGER, description TEXT, notes TEXT, date INTEGER, financial_id TEXT, type TEXT, location TEXT, error TEXT, imported_description TEXT, starting_balance_flag INTEGER DEFAULT 0, transferred_id TEXT, sort_order REAL, tombstone INTEGER DEFAULT 0, cleared INTEGER DEFAULT 1, pending INTEGER DEFAULT 0, parent_id TEXT, schedule TEXT, reconciled INTEGER DEFAULT 0);
CREATE TABLE zero_budgets (id TEXT PRIMARY KEY, month INTEGER, category TEXT, amount INTEGER DEFAULT 0, carryover INTEGER DEFAULT 0, goal INTEGER, long_goal INTEGER);
CREATE TABLE reflect_budgets (id TEXT PRIMARY KEY, month INTEGER, category TEXT, amount INTEGER DEFAULT 0, carryover INTEGER DEFAULT 0, goal INTEGER, long_goal INTEGER);
CREATE TABLE rules (id TEXT PRIMARY KEY, stage TEXT, conditions TEXT, actions TEXT, tombstone INTEGER DEFAULT 0, conditions_op TEXT);
CREATE TABLE schedules (id TEXT PRIMARY KEY, rule TEXT, active INTEGER DEFAULT 0, completed INTEGER DEFAULT 0, posts_transaction INTEGER DEFAULT 0, tombstone INTEGER DEFAULT 0, name TEXT);
CREATE TABLE schedules_next_date (id TEXT PRIMARY KEY, schedule_id TEXT, local_next_date INTEGER, local_next_date_ts INTEGER, base_next_date INTEGER, base_next_date_ts INTEGER, tombstone INTEGER DEFAULT 0);
CREATE TABLE notes (id TEXT PRIMARY KEY, note TEXT);
CREATE TABLE tags (id TEXT PRIMARY KEY, tag TEXT, color TEXT, description TEXT, tombstone INTEGER DEFAULT 0, hidden INTEGER DEFAULT 0);
CREATE TABLE messages_crdt (id INTEGER PRIMARY KEY, timestamp TEXT NOT NULL UNIQUE, dataset TEXT NOT NULL, row TEXT NOT NULL, column TEXT NOT NULL, value BLOB NOT NULL);
`

// Fixture IDs for assertions.
const (
	AcctChecking = "acct-checking"
	AcctCard     = "acct-card"
	AcctSavings  = "acct-savings"
	AcctBroker   = "acct-broker" // off-budget

	GroupBills  = "grp-bills"
	GroupLiving = "grp-living"
	GroupIncome = "grp-income"

	CatRent      = "cat-rent"
	CatGroceries = "cat-groceries"
	CatDining    = "cat-dining"
	CatCar       = "cat-car"
	CatSalary    = "cat-salary"
	CatOldFood   = "cat-oldfood" // deleted, mapped to groceries

	PayeeKroger    = "pay-kroger"
	PayeeAmazon    = "pay-amazon"
	PayeeAmazon2   = "pay-amazon2" // spelling variant of PayeeAmazon
	PayeeAmazon3   = "pay-amazon3" // another variant
	PayeeLandlord  = "pay-landlord"
	PayeeEmployer  = "pay-employer"
	PayeeChipotle  = "pay-chipotle"
	PayeeMerged    = "pay-krogerold" // merged into PayeeKroger via payee_mapping
	PayeeToSavings = "pay-xfer-savings"
	PayeeToCard    = "pay-xfer-card"
	PayeeNetflix   = "pay-netflix"

	RuleKroger  = "rule-kroger"
	RuleDead    = "rule-dead"
	RuleDangle  = "rule-dangling"
	RuleShadow  = "rule-shadow"
	RuleRent    = "rule-rent-sched"
	RuleNetflix = "rule-netflix-sched"

	SchedRent    = "sched-rent"
	SchedNetflix = "sched-netflix"

	TxnDupA = "txn-dup-a"
	TxnDupB = "txn-dup-b"
)

// Build creates a fixture budget database at path. The data spans
// 2026-07 .. 2026-09 so tests can use fixed months.
func Build(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("creating fixture schema: %w", err)
	}
	stmts := []string{
		`INSERT INTO accounts (id,name,offbudget,closed,sort_order) VALUES
			('acct-checking','Checking',0,0,1),('acct-card','Visa Card',0,0,2),('acct-savings','Savings',0,0,3),('acct-broker','Brokerage',1,0,4),('acct-gone','Old Account',0,1,5)`,
		`UPDATE accounts SET tombstone=1 WHERE id='acct-gone'`,
		`INSERT INTO category_groups (id,name,is_income,sort_order) VALUES ('grp-bills','Bills',0,1),('grp-living','Living',0,2),('grp-income','Income',1,3)`,
		`INSERT INTO categories (id,name,is_income,cat_group,sort_order,tombstone,goal_def) VALUES
			('cat-rent','Rent',0,'grp-bills',1,0,NULL),('cat-groceries','Groceries',0,'grp-living',1,0,NULL),
			('cat-dining','Dining Out',0,'grp-living',2,0,NULL),('cat-car','Car Fund',0,'grp-bills',2,0,'[{"type":"goal","amount":6000}]'),
			('cat-salary','Salary',1,'grp-income',1,0,NULL),('cat-oldfood','Food (old)',0,'grp-living',3,1,NULL)`,
		`INSERT INTO category_mapping (id,transferId) VALUES ('cat-rent','cat-rent'),('cat-groceries','cat-groceries'),('cat-dining','cat-dining'),('cat-car','cat-car'),('cat-salary','cat-salary'),('cat-oldfood','cat-groceries')`,
		`INSERT INTO payees (id,name,transfer_acct,tombstone) VALUES
			('pay-kroger','Kroger',NULL,0),('pay-amazon','Amazon',NULL,0),('pay-amazon2','AMAZON MKTPLACE',NULL,0),('pay-amazon3','Amazon.com',NULL,0),
			('pay-landlord','Oak Street Apartments',NULL,0),('pay-employer','Acme Corp Payroll',NULL,0),('pay-chipotle','Chipotle',NULL,0),
			('pay-krogerold','KROGER #123',NULL,1),('pay-xfer-savings','',  'acct-savings',0),('pay-xfer-card','','acct-card',0),
			('pay-netflix','Netflix',NULL,0),('pay-unused','Never Used Inc',NULL,0)`,
		`INSERT INTO payee_mapping (id,targetId) VALUES ('pay-kroger','pay-kroger'),('pay-amazon','pay-amazon'),('pay-amazon2','pay-amazon2'),('pay-amazon3','pay-amazon3'),
			('pay-landlord','pay-landlord'),('pay-employer','pay-employer'),('pay-chipotle','pay-chipotle'),('pay-krogerold','pay-kroger'),
			('pay-xfer-savings','pay-xfer-savings'),('pay-xfer-card','pay-xfer-card'),('pay-netflix','pay-netflix'),('pay-unused','pay-unused')`,
		// Starting balances.
		`INSERT INTO transactions (id,acct,category,amount,description,date,starting_balance_flag,sort_order,cleared) VALUES
			('txn-start-chk','acct-checking','cat-salary',500000,NULL,20260630,1,1,1),
			('txn-start-sav','acct-savings',NULL,1000000,NULL,20260630,1,1,1),
			('txn-start-brk','acct-broker',NULL,2500000,NULL,20260630,1,1,1)`,
		// Income and rent each month.
		`INSERT INTO transactions (id,acct,category,amount,description,date,sort_order,schedule,cleared) VALUES
			('txn-pay-07','acct-checking','cat-salary',400000,'pay-employer',20260701,2,NULL,1),
			('txn-pay-08','acct-checking','cat-salary',400000,'pay-employer',20260801,2,NULL,1),
			('txn-pay-09','acct-checking','cat-salary',400000,'pay-employer',20260901,2,NULL,1),
			('txn-rent-07','acct-checking','cat-rent',-150000,'pay-landlord',20260703,3,'sched-rent',1),
			('txn-rent-08','acct-checking','cat-rent',-150000,'pay-landlord',20260803,3,'sched-rent',1),
			('txn-rent-09','acct-checking','cat-rent',-165000,'pay-landlord',20260903,3,'sched-rent',1)`,
		// Groceries: Kroger history (including a merged payee id) categorizes as groceries.
		`INSERT INTO transactions (id,acct,category,amount,description,notes,imported_description,date,sort_order,cleared) VALUES
			('txn-kr-1','acct-card','cat-groceries',-8450,'pay-kroger',NULL,'KROGER #123 CINCINNATI',20260705,4,1),
			('txn-kr-2','acct-card','cat-groceries',-9120,'pay-kroger','weekly shop','KROGER #123 CINCINNATI',20260712,4,1),
			('txn-kr-3','acct-card','cat-oldfood',-7600,'pay-krogerold',NULL,'KROGER #123',20260719,4,1),
			('txn-kr-4','acct-card','cat-groceries',-10230,'pay-kroger',NULL,'KROGER #123 CINCINNATI',20260809,4,1),
			('txn-kr-5','acct-card','cat-dining',-1500,'pay-kroger','deli lunch','KROGER #123 CINCINNATI',20260816,4,1),
			('txn-kr-6','acct-card',NULL,-8800,'pay-kroger',NULL,'KROGER #123 CINCINNATI',20260906,4,0),
			('txn-ch-1','acct-card','cat-dining',-1845,'pay-chipotle','burrito night',NULL,20260808,5,1),
			('txn-ch-2','acct-card',NULL,-2210,'pay-chipotle',NULL,'CHIPOTLE 0921',20260912,5,0),
			('txn-am-1','acct-card','cat-groceries',-3999,'pay-amazon',NULL,'AMZN Mktp US*2K4',20260714,6,1),
			('txn-am-2','acct-card',NULL,-2599,'pay-amazon2',NULL,'AMAZON MKTPLACE PMTS',20260822,6,1),
			('txn-am-3','acct-card','cat-groceries',-1250,'pay-amazon3',NULL,'Amazon.com*AB12',20260910,6,0),
			('txn-dup-a','acct-card',NULL,-4210,'pay-chipotle',NULL,'CHIPOTLE 0921',20260915,7,0),
			('txn-dup-b','acct-card',NULL,-4210,'pay-chipotle',NULL,'CHIPOTLE 0921',20260916,7,0),
			('txn-netflix-08','acct-card','cat-dining',-1549,'pay-netflix',NULL,'NETFLIX.COM',20260810,8,1),
			('txn-deleted','acct-card','cat-groceries',-99999,'pay-kroger',NULL,NULL,20260901,9,1)`,
		`UPDATE transactions SET tombstone=1 WHERE id='txn-deleted'`,
		// Split: one Amazon order split across groceries and car fund.
		`INSERT INTO transactions (id,acct,category,amount,description,date,isParent,isChild,parent_id,sort_order,cleared) VALUES
			('txn-split','acct-card',NULL,-6000,'pay-amazon',20260820,1,0,NULL,10,1),
			('txn-split-1','acct-card','cat-groceries',-2000,'pay-amazon',20260820,0,1,'txn-split',10,1),
			('txn-split-2','acct-card','cat-car',-4000,'pay-amazon',20260820,0,1,'txn-split',10,1)`,
		// Transfer checking -> savings, and card payment.
		`INSERT INTO transactions (id,acct,category,amount,description,date,transferred_id,sort_order,cleared) VALUES
			('txn-xfer-out','acct-checking',NULL,-50000,'pay-xfer-savings',20260815,'txn-xfer-in',11,1),
			('txn-xfer-in','acct-savings',NULL,50000,'pay-xfer-checking',20260815,'txn-xfer-out',11,1),
			('txn-cardpay-out','acct-checking',NULL,-40000,'pay-xfer-card',20260825,'txn-cardpay-in',12,1),
			('txn-cardpay-in','acct-card',NULL,40000,'pay-xfer-checking',20260825,'txn-cardpay-out',12,1)`,
		`INSERT INTO payees (id,name,transfer_acct) VALUES ('pay-xfer-checking','','acct-checking')`,
		`INSERT INTO payee_mapping (id,targetId) VALUES ('pay-xfer-checking','pay-xfer-checking')`,
		// Budgets (envelope): Sept.
		`INSERT INTO zero_budgets (id,month,category,amount) VALUES
			('202609-cat-rent',202609,'cat-rent',150000),('202609-cat-groceries',202609,'cat-groceries',40000),
			('202609-cat-dining',202609,'cat-dining',10000),('202609-cat-car',202609,'cat-car',20000),
			('202608-cat-rent',202608,'cat-rent',150000),('202608-cat-groceries',202608,'cat-groceries',40000),('202608-cat-dining',202608,'cat-dining',5000)`,
		// Notes with templates.
		`INSERT INTO notes (id,note) VALUES ('cat-groceries','Weekly food\n#template 400'),('cat-dining','#template 150'),('cat-rent','#template 1500'),('budget-202609','September plan')`,
		// Rules: one live Kroger rule, one that never matches, one dangling, one shadowed duplicate.
		`INSERT INTO rules (id,stage,conditions_op,conditions,actions) VALUES
			('rule-kroger',NULL,'and','[{"op":"is","field":"payee","value":"pay-kroger","type":"id"}]','[{"op":"set","field":"category","value":"cat-groceries","type":"id"}]'),
			('rule-dead',NULL,'and','[{"op":"contains","field":"imported_payee","value":"STARBUCKS","type":"string"}]','[{"op":"set","field":"category","value":"cat-dining","type":"id"}]'),
			('rule-dangling',NULL,'and','[{"op":"is","field":"payee","value":"pay-krogerold","type":"id"}]','[{"op":"set","field":"category","value":"cat-oldfood","type":"id"}]'),
			('rule-shadow',NULL,'and','[{"op":"is","field":"payee","value":"pay-kroger","type":"id"}]','[{"op":"set","field":"category","value":"cat-dining","type":"id"}]'),
			('rule-rent-sched',NULL,'and','[{"op":"is","field":"payee","value":"pay-landlord","type":"id"},{"op":"is","field":"account","value":"acct-checking","type":"id"},{"op":"is","field":"amount","value":-150000,"type":"number"},{"op":"isapprox","field":"date","value":{"start":"2026-07-03","frequency":"monthly","interval":1,"endMode":"never"},"type":"date"}]','[{"op":"link-schedule","value":"sched-rent"}]'),
			('rule-netflix-sched',NULL,'and','[{"op":"is","field":"payee","value":"pay-netflix","type":"id"},{"op":"isapprox","field":"amount","value":-1549,"type":"number"},{"op":"isapprox","field":"date","value":{"start":"2026-08-10","frequency":"monthly","interval":1,"endMode":"never"},"type":"date"}]','[{"op":"link-schedule","value":"sched-netflix"}]')`,
		`INSERT INTO schedules (id,rule,active,completed,posts_transaction,name) VALUES ('sched-rent','rule-rent-sched',1,0,0,'Rent'),('sched-netflix','rule-netflix-sched',1,0,0,'Netflix')`,
		`INSERT INTO schedules_next_date (id,schedule_id,local_next_date,base_next_date) VALUES ('snd-rent','sched-rent',20261003,20261003),('snd-netflix','sched-netflix',20260910,20260910)`,
		`INSERT INTO tags (id,tag,color) VALUES ('tag-1','groceries','#00ff00')`,
		// CRDT log: a few changes in September.
		`INSERT INTO messages_crdt (timestamp,dataset,row,column,value) VALUES
			('2026-09-15T10:00:00.000Z-0000-aaaaaaaaaaaaaaaa','transactions','txn-dup-a','amount','N:-4210'),
			('2026-09-15T10:00:00.001Z-0000-aaaaaaaaaaaaaaaa','transactions','txn-dup-a','description','S:pay-chipotle'),
			('2026-09-16T09:30:00.000Z-0000-bbbbbbbbbbbbbbbb','transactions','txn-rent-09','amount','N:-165000'),
			('2026-09-17T12:00:00.000Z-0000-bbbbbbbbbbbbbbbb','transactions','txn-deleted','tombstone','N:1'),
			('2026-09-18T08:00:00.000Z-0000-aaaaaaaaaaaaaaaa','categories','cat-car','name','S:Car Fund'),
			('2026-09-18T08:00:01.000Z-0000-aaaaaaaaaaaaaaaa','zero_budgets','202609-cat-car','amount','N:20000'),
			('2026-08-01T00:00:00.000Z-0000-aaaaaaaaaaaaaaaa','payees','pay-chipotle','name','S:Chipotle')`,
	}
	for i, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("fixture statement %d: %w", i, err)
		}
	}
	return nil
}
