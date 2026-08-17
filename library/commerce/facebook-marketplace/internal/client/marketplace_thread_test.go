package client

import "testing"

func TestParseMarketplaceThreadSummary(t *testing.T) {
	t.Parallel()

	shell := `
<title>Messenger</title>
deleteThenInsertThread",[19,"1784924049824"],[19,"1784916026901"],"Meghana Reddy is waiting for your response about Technivorm Moccamaster KBGV Select \u2013 Yellow Pepper.","Meghana Reddy \u00b7 Technivorm Moccamaster KBGV Select \u2013 Yellow Pepper","https:\/\/img.example\/listing.jpg",false,[19,"80"],[19,"27799138726406206"],[19,"0"],[19,"5"],"inbox","\/messaging\/lightspeed\/media_fallback\/?entity_id=27799138726406206",[19,"1787522555"],[19,"0"],[19,"0"],[19,"0"],[19,"0"],true,[19,"100017629485378"],[9],[9],[9],[9],[9],[9],[9],[9],[9],[19,"0"],[19,"0"],[9],[9],[9],[9],[9],[19,"-12"],[9],[9],[9],[9],[9],[9],false,false,false,[19,"5"],[9],[9],[9],[19,"0"],[9],[19,"0"],false,[9],"michael \u00b7 technivorm moccamaster kbgv select \u2013 yellow pepper",[9],[9],false,[9],[19,"0"],[9],[19,"0"],[9],[19,"27799138726406206"],[9],[19,"-1"],[19,"1"],[19,"0"],"",[19,"0"],[9],[19,"0"],[9],[9],[9],[9],[9],[19,"2"],[9],[9],[9],[19,"8102337"],[19,"4747"],[19,"0"],[9],[19,"3"],[9],"{\"eb_message_timestamp_type\":[{\"timestamp_status\":0}]",[19,"0"],[19,"2"]]
`

	summary := parseMarketplaceThreadSummary(shell, "27799138726406206")
	if summary == nil {
		t.Fatal("expected summary")
	}
	if summary.Snippet != "Meghana Reddy is waiting for your response about Technivorm Moccamaster KBGV Select – Yellow Pepper." {
		t.Fatalf("unexpected snippet: %q", summary.Snippet)
	}
	if summary.Title != "Meghana Reddy · Technivorm Moccamaster KBGV Select – Yellow Pepper" {
		t.Fatalf("unexpected title: %q", summary.Title)
	}
	if summary.FolderCode != 0 {
		t.Fatalf("unexpected folder code: %d", summary.FolderCode)
	}
}

func TestParseMarketplaceThreadMessages(t *testing.T) {
	t.Parallel()

	shell := `
verifyContactRowExists",[19,"8102337"],[19,"1"],"https:\/\/img.example\/me.jpg","Michael Galpert",[19,"1"]
verifyContactRowExists",[19,"100017629485378"],[19,"1"],"https:\/\/img.example\/seller.jpg","Meghana Reddy Bobbili",[19,"1"]
upsertMessage","You started this chat.",[9],[19,"80"],[19,"27799138726406206"],[19,"0"],[19,"1784916026554"],[19,"1784916026554"],[9],"mid.start","7486480428977688798",[19,"8102337"],[9],true
upsertMessage","Hi, I am Michaels AI Chief of Staff helping him buy this. He can do $155 cash and pick up quickly if that works for you.",[9],[19,"80"],[19,"27799138726406206"],[19,"0"],[19,"1784916026901"],[19,"1784916026901"],[9],"mid.offer","7486480430059965337",[19,"8102337"],[9],false
upsertMessage","Hi \u0040Michael Galpert \nLast we can do for 300",[9],[19,"80"],[19,"27799138726406206"],[19,"0"],[19,"1784916092783"],[19,"1784916092783"],[9],"mid.counter","7486480705939133371",[19,"100017629485378"],[9],false
upsertMessage","Hi \u0040Michael Galpert \nLast we can do for 300",[9],[19,"80"],[19,"27799138726406206"],[19,"0"],[19,"1784916092783"],[19,"1784916092783"],[9],"mid.counter","7486480705939133371",[19,"100017629485378"],[9],false
upsertMessage","Meghana Reddy is waiting for your response about Technivorm Moccamaster KBGV Select \u2013 Yellow Pepper.",[9],[19,"80"],[19,"27799138726406206"],[19,"0"],[19,"1784924049824"],[19,"1784924049824"],[9],"mid.reminder","7486514080953655043",[19,"100017629485378"],[9],true
upsertMessage","ignore this other thread",[9],[19,"80"],[19,"999"],[19,"0"],[19,"1"],[19,"1"],[9],"mid.other","1",[19,"100017629485378"],[9],false
`

	contacts := parseMarketplaceThreadContacts(shell)
	messages := parseMarketplaceThreadMessages(shell, "27799138726406206", contacts)
	if len(messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(messages))
	}
	if messages[2].Text != "Hi @Michael Galpert \nLast we can do for 300" {
		t.Fatalf("unexpected counter text: %q", messages[2].Text)
	}
	if messages[2].SenderName != "Meghana Reddy Bobbili" {
		t.Fatalf("unexpected sender name: %q", messages[2].SenderName)
	}
	if messages[3].MessageID != "mid.reminder" {
		t.Fatalf("unexpected latest message id: %q", messages[3].MessageID)
	}
}
