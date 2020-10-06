package log_code

const (
	MdTypeData = "typeData"
	MdMethod   = "method"
)

var (
	TypeData = typeData{
		SendMultipart: "sendMultipart",
		GetFile:       "getFile",
		JsonContent:   "jsonContent",
	}
)

type (
	typeData struct {
		SendMultipart string
		GetFile       string
		JsonContent   string
	}
)
