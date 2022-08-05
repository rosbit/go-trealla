package trealla

// make any structrue to a record like table(field1, field2, ..., fieldN)
// implementation of Record can be used as a arg when calling Query()
type Record interface {
	TableName() string
	FieldValues() []interface{}
}

