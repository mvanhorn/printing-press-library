package cli

import (
	"testing"
)

func TestListsNotificationCommandsAreRegistered(t *testing.T) {
	root := RootCmd()
	for _, path := range [][]string{{"lists", "notifications"}, {"lists", "notifications", "add"}, {"lists", "notifications", "remove"}} {
		if command, _, err := root.Find(path); err != nil || command == nil {
			t.Fatalf("Find(%v) = %#v, %v", path, command, err)
		}
	}
	notifications, _, _ := root.Find([]string{"lists", "notifications"})
	if notifications.Hidden {
		t.Fatal("notifications command must be visible")
	}
	for _, name := range []string{"add", "remove"} {
		command, _, _ := root.Find([]string{"lists", "notifications", name})
		if command.Flags().Lookup("apply") == nil {
			t.Fatalf("%s command is missing --apply", name)
		}
	}
}

func TestListsNotificationAddPreviewDoesNotRequireCredentials(t *testing.T) {
	cmd := newListsNotificationsAddCmd(&rootFlags{})
	cmd.SetArgs([]string{"--list", "Groceries", "--location-name", "Home", "--address", "123 Test Avenue", "--latitude", "33.6", "--longitude", "-95.5"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
}

func TestListsNotificationApplyValidatesBeforeNetwork(t *testing.T) {
	cmd := newListsNotificationsAddCmd(&rootFlags{})
	cmd.SetArgs([]string{"--list", "Groceries", "--location-name", "Home", "--address", "123 Test Avenue", "--apply"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("apply accepted missing coordinates")
	}

	remove := newListsNotificationsRemoveCmd(&rootFlags{})
	remove.SetArgs([]string{"--list", "Groceries", "--apply"})
	if err := remove.Execute(); err == nil {
		t.Fatal("remove accepted missing location selector")
	}
}
