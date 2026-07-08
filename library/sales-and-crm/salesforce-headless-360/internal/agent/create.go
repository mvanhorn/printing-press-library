package agent

func NewCreateWriteOptions(sobject, idempotencyKey string, fields map[string]any) WriteOptions {
	return WriteOptions{
		Operation:      WriteOperationCreate,
		SObject:        sobject,
		IdempotencyKey: idempotencyKey,
		Fields:         fields,
	}
}
