package types


// Assignment holds a signature and a filename which are needed to upload a
// file and validate it. Since mapstructure doesn't support embedded structs
// we have to copy these to main.go's uploadArgs
type Assignment struct {
	Signature string `json:"sig" mapstructure:"sig"  validate:"nonzero"`
	Filename  string `json:"filename"  mapstructure:"filename" validate:"nonzero"`
}
