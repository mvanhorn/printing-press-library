package agent

func NewUpsertWriteOptions(sobject, idempotencyKey string, fields map[string]any) WriteOptions {
	return WriteOptions{
		Operation:      WriteOperationUpsert,
		SObject:        sobject,
		IdempotencyKey: idempotencyKey,
		Fields:         fields,
	}
}
