package cli

import "testing"

func TestContainsMutationOperation(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{
			name: "query operation",
			query: `query GetAccounts {
  accounts { id }
}`,
		},
		{
			name: "anonymous selection with field containing mutation",
			query: `{
  mutationStatus
  notes(text: "mutation")
}`,
		},
		{
			name: "comment containing mutation",
			query: `# mutation is discussed here
query GetTags {
  householdTransactionTags { id }
}`,
		},
		{
			name: "block string containing mutation",
			query: `query GetNotes {
  notes(text: """mutation example""")
}`,
		},
		{
			name: "mutation operation",
			query: `mutation UpdateSomething {
  updateTransaction { id }
}`,
			want: true,
		},
		{
			name: "second top level mutation operation",
			query: `query GetAccounts { accounts { id } }
mutation UpdateSomething { updateTransaction { id } }`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsMutationOperation(tt.query); got != tt.want {
				t.Fatalf("containsMutationOperation() = %v, want %v", got, tt.want)
			}
		})
	}
}
