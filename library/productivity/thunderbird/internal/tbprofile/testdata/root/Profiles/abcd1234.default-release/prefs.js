// Mozilla User Preferences

// Fictional fixture profile for tests.
user_pref("mail.accountmanager.accounts", "account1,account2");
user_pref("mail.account.account1.identities", "id1,id3");
user_pref("mail.account.account1.server", "server1");
user_pref("mail.account.account2.server", "server2");
user_pref("mail.identity.id1.fullName", "Bob Example");
user_pref("mail.identity.id1.useremail", "bob@example.com");
user_pref("mail.identity.id3.fullName", "Bob \"Work\" Example");
user_pref("mail.identity.id3.useremail", "bob.work@example.com");
user_pref("mail.server.server1.directory", "C:\\nonexistent\\ImapMail\\imap.example.com");
user_pref("mail.server.server1.directory-rel", "[ProfD]ImapMail/imap.example.com");
user_pref("mail.server.server1.hostname", "imap.example.com");
user_pref("mail.server.server1.name", "bob@example.com");
user_pref("mail.server.server1.type", "imap");
user_pref("mail.server.server1.userName", "bob@example.com");
user_pref("mail.server.server2.directory-rel", "[ProfD]Mail/Local Folders");
user_pref("mail.server.server2.hostname", "Local Folders");
user_pref("mail.server.server2.name", "Local Folders");
user_pref("mail.server.server2.type", "none");
user_pref("mail.server.server2.userName", "nobody");
user_pref("mail.fixture.count", 3);
user_pref("mail.fixture.flag", true);
