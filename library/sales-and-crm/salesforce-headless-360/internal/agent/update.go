package agent

func NewUpdateWriteOptions(recordID string, fields map[string]any) WriteOptions {
	return WriteOptions{
		Operation: WriteOperationUpdate,
		RecordID:  recordID,
		SObject:   InferSObjectFromID(recordID),
		Fields:    fields,
	}
}
